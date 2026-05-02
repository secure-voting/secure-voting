export const env = {
  frontendBase: process.env.FRONTEND_BASE || "https://127.0.0.1:8080",
  backendBase: process.env.BACKEND_BASE || "http://127.0.0.1:3001",
  apiBase:
    process.env.API_BASE ||
    `${process.env.FRONTEND_BASE || "https://127.0.0.1:8080"}/api/v1`,

  adminEmail: process.env.BOOTSTRAP_ADMIN_EMAIL || "admin@example.com",
  adminPassword: process.env.BOOTSTRAP_ADMIN_PASSWORD || "AdminPass123!",

  researcherEmail: process.env.BOOTSTRAP_RESEARCHER_EMAIL || "researcher@example.com",
  researcherPassword: process.env.BOOTSTRAP_RESEARCHER_PASSWORD || "ResearchPass123!",
};