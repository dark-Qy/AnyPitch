import { afterEach, describe, expect, it, vi } from "vitest";
import { APIClient } from "./api";

describe("APIClient", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("deletes coach events through the event detail endpoint", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ data: { ok: true } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const client = new APIClient("coach-token");
    await client.deleteEvent("event-1");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/events/event-1",
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Authorization: "Bearer coach-token",
        }),
      }),
    );
  });
});
