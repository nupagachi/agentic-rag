// Liveness probe for Docker/K8s; not routed through Nginx /api.
export const dynamic = "force-dynamic";

export function GET() {
  return Response.json({ status: "ok" });
}
