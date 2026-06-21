import client from "./client";
import type {
  APIToken,
  APITokenCreated,
  CreateAPITokenRequest,
} from "@/types/tokens";

export async function listAPITokens(): Promise<APIToken[]> {
  const res = await client.get("/auth/tokens");
  return res.data;
}

export async function createAPIToken(
  payload: CreateAPITokenRequest,
): Promise<APITokenCreated> {
  const res = await client.post("/auth/tokens", payload);
  return res.data;
}

export async function deleteAPIToken(id: number): Promise<void> {
  await client.delete(`/auth/tokens/${id}`);
}
