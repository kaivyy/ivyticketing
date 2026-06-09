import { defineConfig } from "astro/config";
import tailwind from "@astrojs/tailwind";
import node from "@astrojs/node";

export default defineConfig({
  integrations: [tailwind()],
  output: "hybrid",
  adapter: node({ mode: "standalone" }),
  server: { port: 4321 },
  redirects: {
    "/organizations/[orgId]/events/[eventId]/queue-controls":
      "/org/[orgId]/events/[eventId]/queue-controls",
    "/organizations/[orgId]/events/[eventId]/corporate":
      "/org/[orgId]/events/[eventId]/corporate",
  },
});
