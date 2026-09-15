import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import { api, setCSRF, type Session } from "./api";
const SessionContext = createContext<{
  session: Session;
  loading: boolean;
  error: string;
  refresh: () => Promise<void>;
  accept: (s: Session) => void;
}>({
  session: { user: null, csrf_token: "" },
  loading: true,
  error: "",
  refresh: async () => {},
  accept: () => {},
});
export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<Session>({
    user: null,
    csrf_token: "",
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const accept = useCallback((s: Session) => {
    setCSRF(s.csrf_token);
    setSession(s);
    setError("");
  }, []);
  const refresh = useCallback(async () => {
    try {
      accept(await api<Session>("/me"));
    } catch {
      setError("Не удалось подключиться к серверу.");
    } finally {
      setLoading(false);
    }
  }, [accept]);
  useEffect(() => {
    void refresh();
  }, [refresh]);
  return (
    <SessionContext.Provider
      value={{ session, loading, error, refresh, accept }}
    >
      {children}
    </SessionContext.Provider>
  );
}
export function useSession() {
  return useContext(SessionContext);
}
