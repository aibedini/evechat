import { describe, expect, it } from "vitest";
import { storageGet, storageRemove, storageSet } from "./config";

// In the node test environment there is no localStorage at all, which is exactly
// the private-mode / file:// case the widget has to survive.
describe("storage fallback", () => {
  it("keeps values in memory when localStorage is unavailable", () => {
    storageSet("evechat:test:key", "value");
    expect(storageGet("evechat:test:key")).toBe("value");
  });

  it("removes values from the memory fallback", () => {
    storageSet("evechat:test:gone", "x");
    storageRemove("evechat:test:gone");
    expect(storageGet("evechat:test:gone")).toBeNull();
  });

  it("returns null for unknown keys", () => {
    expect(storageGet("evechat:test:missing")).toBeNull();
  });
});
