#!/usr/bin/env -S node --loader ts-node/esm

import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import OpenAI from "openai";

type JSONValue = Record<string, any>;

function parseArgs() {
  const args = process.argv.slice(2);
  const out: Record<string, any> = {
    task: "code",
    allowFallback: false,
    dryRun: false,
    inferctl: process.env.INFERCTL_BIN || "inferctl",
  };
  const promptParts: string[] = [];
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === "--task") out.task = args[++i];
    else if (arg === "--prompt-file") out.promptFile = args[++i];
    else if (arg === "--allow-fallback") out.allowFallback = true;
    else if (arg === "--dry-run") out.dryRun = true;
    else if (arg === "--inferctl") out.inferctl = args[++i];
    else promptParts.push(arg);
  }
  out.prompt = promptParts.join(" ");
  if (out.prompt && out.promptFile) throw new Error("use either prompt text or --prompt-file, not both");
  if (!out.prompt && !out.promptFile) throw new Error("provide prompt text or --prompt-file");
  return out;
}

function inferctlJSON(binary: string, args: string[]): JSONValue {
  const proc = spawnSync(binary, [...args, "--json"], { encoding: "utf8" });
  let envelope: JSONValue;
  try {
    envelope = JSON.parse(proc.stdout);
  } catch {
    const detail = (proc.stderr || proc.stdout || "").trim();
    if (args.includes("preflight")) {
      throw new Error(`this demo requires an inferctl build with preflight; ${detail}`);
    }
    throw new Error(`inferctl ${args.join(" ")} did not emit JSON: ${detail}`);
  }
  if (proc.status !== 0 || envelope.ok !== true) {
    const first = envelope.errors?.[0];
    throw new Error(`inferctl ${args.join(" ")} failed: ${first?.message || first?.code || proc.stderr}`);
  }
  return envelope;
}

function promptFileFromArgs(args: Record<string, any>): [string, () => void] {
  if (args.promptFile) return [args.promptFile, () => {}];
  const dir = mkdtempSync(join(tmpdir(), "inferctl-agent-"));
  const file = join(dir, "prompt.txt");
  writeFileSync(file, args.prompt, "utf8");
  return [file, () => rmSync(dir, { recursive: true, force: true })];
}

function selectedBaseURL(config: JSONValue, backend: string): string {
  const row = config.data?.effective_config?.backends?.[backend];
  if (!row?.base_url) throw new Error(`selected backend ${backend} is missing from config show output`);
  return row.base_url;
}

function openAIBaseURL(raw: string): string {
  const url = new URL(raw);
  const path = url.pathname.replace(/\/+$/, "");
  url.pathname = path.endsWith("/v1") || path === "/v1" ? path : `${path}/v1`;
  return url.toString().replace(/\/$/, "");
}

async function main() {
  const args = parseArgs();
  const [promptFile, cleanup] = promptFileFromArgs(args);
  try {
    const preflightArgs = ["preflight", args.task, "--prompt-file", promptFile];
    if (args.allowFallback) preflightArgs.push("--allow-fallback");
    const preflight = inferctlJSON(args.inferctl, preflightArgs);
    const config = inferctlJSON(args.inferctl, ["config", "show"]);
    const decision = preflight.data.route_decision;
    const backend = decision.selected_backend;
    const model = decision.selected_model;
    const apiBase = openAIBaseURL(selectedBaseURL(config, backend));

    console.log(`inferctl selected: ${backend} / ${model}`);
    console.log(`reason: ${decision.reason}`);
    console.log(`data plane: calling ${apiBase} directly`);
    console.log("---");

    if (args.dryRun) {
      console.log("[dry run: backend call skipped]");
      return;
    }

    const client = new OpenAI({
      baseURL: apiBase,
      apiKey: process.env.OPENAI_API_KEY || "placeholder",
    });
    const response = await client.chat.completions.create({
      model,
      messages: [{ role: "user", content: readFileSync(promptFile, "utf8") }],
    });
    console.log(response.choices[0]?.message?.content || "");
  } finally {
    cleanup();
  }
}

main().catch((err) => {
  console.error(`error: ${err.message}`);
  process.exit(1);
});
