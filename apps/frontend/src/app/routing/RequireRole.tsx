import React from "react";
import { Navigate } from "react-router-dom";
import { useAuth } from "../auth";

export function RequireRole({ role, children }: { role: "admin" | "voter" | "researcher"; children: React.ReactNode }) {
  const { me } = useAuth();
  if (!me?.role) return <Navigate to="/elections" replace />;
  if (me.role !== role) return <Navigate to="/elections" replace />;
  return <>{children}</>;
}