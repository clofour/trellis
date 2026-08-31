export async function register() {
  if (process.env.NEXT_RUNTIME !== "nodejs") return;
  const caCert = process.env.TRELLIS_CA_CERT;
  if (!caCert) return;
  const { Agent, setGlobalDispatcher } = await import("undici");
  setGlobalDispatcher(new Agent({ connect: { ca: caCert } }));
}
