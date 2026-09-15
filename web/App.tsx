import {
  Link,
  NavLink,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router-dom";
import {
  CalendarDays,
  LayoutDashboard,
  LogIn,
  LogOut,
  ScanLine,
  Ticket,
  UserRound,
} from "lucide-react";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { Button } from "@/components/ui/button";
import { Toaster } from "@/components/ui/sonner";
import { toast } from "sonner";
import { post, message } from "./api";
import { useSession } from "./session";
import { Empty, ErrorBox, Loading } from "./shared";
import { Catalog, EventDetails } from "./events";
import { Auth, Profile, Join } from "./account";
import { EventForm } from "./event-form";
import { Manage } from "./manage";
import { MyTickets, TicketDetail } from "./tickets";
import { Control } from "./control";
import { useWebMCP } from "./webmcp";
const nav = [
  { to: "/", label: "Афиша", icon: CalendarDays },
  { to: "/tickets", label: "Мои пропуска", icon: Ticket },
  { to: "/manage", label: "Мои мероприятия", icon: LayoutDashboard },
  { to: "/control", label: "Контроль входа", icon: ScanLine },
];
function Protected({ children }: { children: React.ReactNode }) {
  const { session, loading } = useSession();
  const location = useLocation();
  if (loading) return <Loading />;
  if (!session.user)
    return (
      <Empty
        title="Войдите в аккаунт"
        description="Ваши мероприятия и пропуска будут доступны на любом устройстве."
      >
        <Button asChild>
          <Link
            to="/login"
            state={{ returnTo: location.pathname + location.hash }}
          >
            Войти
            <LogIn size={18} />
          </Link>
        </Button>
      </Empty>
    );
  return children;
}
export function App() {
  useWebMCP();
  const { session, loading, error, refresh, accept } = useSession();
  const location = useLocation();
  const navigate = useNavigate();
  const active = nav.find((n) =>
    n.to === "/"
      ? location.pathname === "/" || location.pathname.startsWith("/events/")
      : location.pathname.startsWith(n.to),
  );
  async function signOut() {
    try {
      await post("/auth/logout", {});
      accept({ user: null, csrf_token: "" });
      navigate("/");
      toast.success("Вы вышли из аккаунта");
    } catch (e) {
      toast.error(message(e));
    }
  }
  return (
    <SidebarProvider>
      <Sidebar className="campus-sidebar">
        <SidebarHeader>
          <Link to="/" className="brand">
            <span className="brand-mark">
              <Ticket size={27} />
            </span>
            <span>
              кампус
              <span className="brand-caption">МЕРОПРИЯТИЯ И ПРОПУСКА</span>
            </span>
          </Link>
        </SidebarHeader>
        <SidebarContent>
          <p className="nav-label">ПРОСТРАНСТВО КАМПУСА</p>
          <SidebarMenu>
            {nav.map((n) => (
              <SidebarMenuItem key={n.to}>
                <SidebarMenuButton
                  asChild
                  isActive={active?.to === n.to}
                  tooltip={n.label}
                >
                  <NavLink to={n.to}>
                    <n.icon />
                    <span>{n.label}</span>
                  </NavLink>
                </SidebarMenuButton>
              </SidebarMenuItem>
            ))}
          </SidebarMenu>
          <div className="sidebar-note">
            <span className="note-line" />
            <p>Всё готово к входу.</p>
            <span>Пропуск всегда под рукой.</span>
          </div>
        </SidebarContent>
        <SidebarFooter>
          {session.user ? (
            <>
              <Link to="/profile" className="account-link">
                <span className="avatar">
                  {session.user.name.slice(0, 1).toUpperCase()}
                </span>
                <span>
                  <strong>{session.user.name}</strong>
                  <small>Мой аккаунт</small>
                </span>
                <UserRound size={17} />
              </Link>
              <Button variant="ghost" className="logout" onClick={signOut}>
                <LogOut size={16} />
                Выйти
              </Button>
            </>
          ) : (
            <Button asChild className="sidebar-login">
              <Link to="/login">
                <LogIn size={17} />
                Войти в аккаунт
              </Link>
            </Button>
          )}
        </SidebarFooter>
      </Sidebar>
      <SidebarInset>
        <header className="topbar">
          <div className="topbar-left">
            <SidebarTrigger />
            <span className="breadcrumb">
              Кампус<span>/</span>
              <strong>{active?.label ?? "Аккаунт"}</strong>
            </span>
          </div>
          <span className="topbar-right">
            {session.user ? (
              <>
                <span className="small-avatar">
                  {session.user.name.slice(0, 1).toUpperCase()}
                </span>
                <span>{session.user.name}</span>
              </>
            ) : !loading ? (
              <Link to="/register">Создать аккаунт</Link>
            ) : null}
          </span>
        </header>
        <main
          id="main-content"
          className={`workspace ${location.pathname.startsWith("/control/") ? "scanner-workspace" : ""}`}
        >
          {error && <ErrorBox text={error} retry={() => void refresh()} />}
          <Routes>
            <Route path="/" element={<Catalog />} />
            <Route path="/events/:id" element={<EventDetails />} />
            <Route
              path="/manage"
              element={
                <Protected>
                  <Catalog mode="manage" />
                </Protected>
              }
            />
            <Route
              path="/manage/new"
              element={
                <Protected>
                  <EventForm />
                </Protected>
              }
            />
            <Route
              path="/manage/:id"
              element={
                <Protected>
                  <Manage />
                </Protected>
              }
            />
            <Route
              path="/tickets"
              element={
                <Protected>
                  <MyTickets />
                </Protected>
              }
            />
            <Route
              path="/tickets/:id"
              element={
                <Protected>
                  <TicketDetail />
                </Protected>
              }
            />
            <Route
              path="/control"
              element={
                <Protected>
                  <Catalog mode="control" />
                </Protected>
              }
            />
            <Route
              path="/control/:id"
              element={
                <Protected>
                  <Control />
                </Protected>
              }
            />
            <Route path="/login" element={<Auth />} />
            <Route path="/register" element={<Auth register />} />
            <Route
              path="/profile"
              element={
                <Protected>
                  <Profile />
                </Protected>
              }
            />
            <Route path="/join" element={<Join />} />
            <Route
              path="*"
              element={
                <Empty
                  title="Страница не найдена"
                  description="Возможно, адрес изменился."
                >
                  <Button asChild>
                    <Link to="/">На афишу</Link>
                  </Button>
                </Empty>
              }
            />
          </Routes>
        </main>
        <footer className="app-footer">
          <span>Кампус</span>
          <span>Время мероприятий — московское (UTC+3)</span>
        </footer>
      </SidebarInset>
      <Toaster position="bottom-right" richColors closeButton />
    </SidebarProvider>
  );
}
