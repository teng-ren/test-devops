import { Routes, Route } from "react-router-dom";
import AILoginPage from "./pages/Login.tsx";
import AIWorkspace from "./pages/UserDashboard.tsx";
import AdminPage from "./pages/AdminDashboard.tsx";

function App() {
  return (
    <div>
      <Routes>
        <Route path="/" element={<AILoginPage/>} />
        <Route path="/user" element={<AIWorkspace/>} />
        <Route path="/admin" element={<AdminPage/>} />
      </Routes>
    </div>
  );
}

export default App;

