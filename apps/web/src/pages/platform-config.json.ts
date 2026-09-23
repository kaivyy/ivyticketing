import type { APIRoute } from "astro";
import fs from "node:fs";
import path from "node:path";
import { DEFAULT_HOMEPAGE_CONFIG } from "../lib/events-store";

export const prerender = false;

const POSSIBLE_FILES = [
  path.resolve(process.cwd(), "homepage-config.json"),
  path.resolve(process.cwd(), "apps/web/homepage-config.json"),
  path.resolve(process.cwd(), "../homepage-config.json"),
];

function getConfigFile(): string {
  for (const f of POSSIBLE_FILES) {
    if (fs.existsSync(f)) return f;
  }
  return POSSIBLE_FILES[0];
}

export const GET: APIRoute = async () => {
  try {
    const file = getConfigFile();
    if (fs.existsSync(file)) {
      const data = fs.readFileSync(file, "utf-8");
      const parsed = JSON.parse(data);
      return new Response(JSON.stringify(parsed), {
        status: 200,
        headers: {
          "Content-Type": "application/json",
          "Cache-Control": "no-store, no-cache, must-revalidate",
        },
      });
    }
  } catch {
    // Fallback ke konfigurasi bawaan bila file belum ada atau rusak
  }

  return new Response(JSON.stringify(DEFAULT_HOMEPAGE_CONFIG), {
    status: 200,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "no-store, no-cache, must-revalidate",
    },
  });
};

export const POST: APIRoute = async ({ request }) => {
  try {
    const body = await request.json();
    const content = JSON.stringify(body, null, 2);
    for (const f of POSSIBLE_FILES) {
      try {
        if (fs.existsSync(path.dirname(f))) {
          fs.writeFileSync(f, content, "utf-8");
        }
      } catch {}
    }
    return new Response(JSON.stringify({ ok: true }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  } catch (err: any) {
    return new Response(JSON.stringify({ ok: false, error: String(err?.message || err) }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }
};
