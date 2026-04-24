import React from "react";
import { Navigate } from "react-router-dom";
import { useAuth } from "../auth";

export function RequireAnyRole({
  roles,
  children,
}: {
  roles: Array<"admin" | "voter" | "researcher">;
  children: React.ReactNode;
}) {
  const { me } = useAuth();

  if (!me?.role) return <Navigate to="/dashboard" replace />;
  if (!roles.includes(me.role as "admin" | "voter" | "researcher")) {
    return <Navigate to="/dashboard" replace />;
  }

  return <>{children}</>;
}