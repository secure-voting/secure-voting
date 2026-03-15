import React from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "../auth";

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { authed } = useAuth();
  const loc = useLocation();
  if (!authed) return <Navigate to="/login" replace state={{ from: loc.pathname }} />;
  return <>{children}</>;
}