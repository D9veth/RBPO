package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/D9veth/RBPO/internal/security"
	"github.com/jackc/pgx/v5"
)

type Admission struct {
	Result  string `json:"result"`
	Message string `json:"message"`
	Pass    *Pass  `json:"pass,omitempty"`
	Replay  bool   `json:"replay"`
}

func (a *App) checkIn(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	k, err := key(r)
	if err != nil {
		return err
	}
	var in struct {
		Code string `json:"code"`
	}
	if err = decode(r, &in); err != nil {
		return err
	}
	if err = a.rate(r.Context(), "scan:"+u.ID, 120, time.Minute); err != nil {
		return err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "redeem:"+u.ID+":"+k); err != nil {
		return err
	}
	e, err := a.event(r.Context(), tx, r.PathValue("event"), u.ID, "FOR SHARE OF e")
	if err != nil {
		return err
	}
	if err = permission(e, "controller"); err != nil {
		return err
	}
	code, normalizeErr := security.NormalizeCode(in.Code)
	result := Admission{Result: "invalid", Message: "Проверьте код: он состоит из 32 букв и цифр."}
	status := 422
	var p Pass
	if normalizeErr == nil {
		p, err = scanPass(tx.QueryRow(r.Context(), `SELECT `+passColumns+` FROM passes p JOIN events e ON e.id=p.event_id WHERE p.code_hash=$1`, security.Hash(code)))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			result = Admission{Result: "not_found", Message: "Такой пропуск не найден."}
			status = 404
		case p.EventID != e.ID:
			result = Admission{Result: "wrong_event", Message: "Пропуск выдан на другое мероприятие."}
			status = 409
		default:
			var replayPass string
			replayErr := tx.QueryRow(r.Context(), `SELECT pass_id FROM redemptions WHERE actor_id=$1 AND request_id=$2`, u.ID, k).Scan(&replayPass)
			if replayErr == nil {
				if replayPass != p.ID {
					return problem(409, "IDEMPOTENCY_CONFLICT", "Этот запрос уже использован для другого пропуска.")
				}
				write(w, 200, Admission{Result: "accepted", Message: "Проход уже подтверждён этим запросом.", Pass: &p, Replay: true})
				return nil
			}
			if !errors.Is(replayErr, pgx.ErrNoRows) {
				return replayErr
			}
			switch {
			case e.Status != "published":
				result = Admission{Result: "cancelled", Message: "Вход на это мероприятие закрыт."}
				status = 409
			case time.Now().Before(e.DoorsAt):
				result = Admission{Result: "not_open", Message: "Вход ещё не открыт."}
				status = 409
			case !time.Now().Before(e.EndsAt):
				result = Admission{Result: "closed", Message: "Мероприятие уже закончилось."}
				status = 409
			default:
				var redeemedAt time.Time
				err = tx.QueryRow(r.Context(), `UPDATE passes SET status='redeemed',redeemed_at=now(),redeemed_by=$2 WHERE id=$1 AND status='active' RETURNING redeemed_at`, p.ID, u.ID).Scan(&redeemedAt)
				if err == nil {
					if _, err = tx.Exec(r.Context(), `INSERT INTO redemptions(actor_id,request_id,pass_id,event_id,redeemed_at) VALUES($1,$2,$3,$4,$5)`, u.ID, k, p.ID, e.ID, redeemedAt); err != nil {
						return err
					}
					if err = record(r.Context(), tx, e.ID, u.ID, "pass.redeemed", p.ID, map[string]string{}); err != nil {
						return err
					}
					p.Status = "redeemed"
					p.RedeemedAt = &redeemedAt
					p.RedeemedBy = &u.ID
					result = Admission{Result: "accepted", Message: "Проход разрешён.", Pass: &p}
					status = 200
				} else if errors.Is(err, pgx.ErrNoRows) {
					p, err = a.pass(r, tx, p.ID)
					if err != nil {
						return err
					}
					if p.Status == "redeemed" {
						result = Admission{Result: "already_used", Message: "Этот пропуск уже использован.", Pass: &p}
					} else {
						result = Admission{Result: "revoked", Message: "Пропуск отменён.", Pass: &p}
					}
					status = 409
				} else {
					return err
				}
			}
		}
	}
	var passID *string
	if p.ID != "" && p.EventID == e.ID {
		passID = &p.ID
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO admission_attempts(event_id,actor_id,pass_id,result) VALUES($1,$2,$3,$4)`, e.ID, u.ID, passID, result.Result); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, status, result)
	return nil
}
func (a *App) activity(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	e, err := a.event(r.Context(), a.DB, r.PathValue("event"), u.ID, "")
	if err != nil {
		return err
	}
	if err = permission(e, "controller"); err != nil {
		return err
	}
	rows, err := a.DB.Query(r.Context(), `SELECT a.id,a.result,a.created_at,u.name,COALESCE(p.holder_name,'') FROM admission_attempts a JOIN users u ON u.id=a.actor_id LEFT JOIN passes p ON p.id=a.pass_id WHERE a.event_id=$1 ORDER BY a.id DESC LIMIT 50`, e.ID)
	if err != nil {
		return err
	}
	type attempt struct {
		ID         int64     `json:"id"`
		Result     string    `json:"result"`
		CreatedAt  time.Time `json:"created_at"`
		Actor      string    `json:"actor"`
		HolderName string    `json:"holder_name"`
	}
	attempts := []attempt{}
	for rows.Next() {
		var item attempt
		if err = rows.Scan(&item.ID, &item.Result, &item.CreatedAt, &item.Actor, &item.HolderName); err != nil {
			rows.Close()
			return err
		}
		attempts = append(attempts, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	logs := []map[string]any{}
	if e.Role == "organizer" {
		audit, err := a.DB.Query(r.Context(), `SELECT a.id,a.action,a.target_id,a.details,a.created_at,u.name FROM audit_log a JOIN users u ON u.id=a.actor_id WHERE a.event_id=$1 ORDER BY a.id DESC LIMIT 100`, e.ID)
		if err != nil {
			return err
		}
		defer audit.Close()
		for audit.Next() {
			var id int64
			var action, name string
			var target *string
			var details map[string]any
			var at time.Time
			if err = audit.Scan(&id, &action, &target, &details, &at, &name); err != nil {
				return err
			}
			logs = append(logs, map[string]any{"id": id, "action": action, "target_id": target, "details": details, "created_at": at, "actor": name})
		}
		if err = audit.Err(); err != nil {
			return err
		}
	}
	write(w, 200, map[string]any{"attempts": attempts, "audit": logs, "event": e})
	return nil
}
