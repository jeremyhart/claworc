import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { successToast, errorToast } from "@/utils/toast";
import {
  listAPITokens,
  createAPIToken,
  deleteAPIToken,
} from "@/api/tokens";
import type {
  CreateAPITokenRequest,
} from "@/types/tokens";

export function useAPITokens() {
  return useQuery({
    queryKey: ["api-tokens"],
    queryFn: listAPITokens,
  });
}

export function useCreateAPIToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateAPITokenRequest) => createAPIToken(payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-tokens"] });
    },
    onError: (error: any) => {
      errorToast("Failed to create API token", error);
    },
  });
}

export function useDeleteAPIToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => deleteAPIToken(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-tokens"] });
      successToast("API token revoked");
    },
    onError: (error: any) => {
      errorToast("Failed to revoke API token", error);
    },
  });
}
