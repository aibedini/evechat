import { describe, expect, it } from "vitest";
import { createHash } from "node:crypto";
import { solvePoW } from "./pow";

function leadingZeroBits(hex: string): number {
  let bits = 0;
  for (const ch of hex) {
    const v = parseInt(ch, 16);
    if (v === 0) {
      bits += 4;
      continue;
    }
    if (v < 2) bits += 3;
    else if (v < 4) bits += 2;
    else if (v < 8) bits += 1;
    break;
  }
  return bits;
}

function verify(challenge: string, nonce: string, difficulty: number): boolean {
  const digest = createHash("sha256").update(challenge + nonce).digest("hex");
  return leadingZeroBits(digest) >= difficulty;
}

describe("proof of work", () => {
  it("solves a 12 bit challenge accepted by an independent SHA-256", () => {
    const challenge = "sL2mQ9vK3pR7tX1yZ4bN6cD8fG0hJ5kA";
    const nonce = solvePoW(challenge, 12);
    expect(nonce).not.toBe("");
    expect(verify(challenge, nonce, 12)).toBe(true);
  });

  it("returns an empty nonce when difficulty is disabled", () => {
    expect(solvePoW("challenge", 0)).toBe("");
  });

  it("solves a second challenge", () => {
    const challenge = "9f8e7d6c5b4a39281706f5e4d3c2b1a0";
    const nonce = solvePoW(challenge, 8);
    expect(verify(challenge, nonce, 8)).toBe(true);
  });
});
