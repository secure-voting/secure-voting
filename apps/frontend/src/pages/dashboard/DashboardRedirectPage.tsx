import React from "react";
import { Navigate } from "react-router-dom";
import { useAuth } from "../../app/auth";

export function DashboardRedirectPage() {
  const { me } = useAuth();

  if (me?.role === "admin") {
    return <Navigate to="/dashboard/admin" replace />;
  }

  if (me?.role === "researcher") {
    return <Navigate to="/dashboard/researcher" replace />;
  }

  return <Navigate to="/dashboard/voter" replace />;
}