import { useQuery } from "@tanstack/react-query"
import { request } from "../../../shared/api/api"

export function useDocsAvailability(): boolean {
  const documentation = useQuery({
    queryKey: ["openapi-document-present"],
    queryFn: async () => {
      const result = await request("/openapi.yaml", { expectJson: false })

      return result.ok
    },
    staleTime: Infinity,
    refetchInterval: false,
  })

  return documentation.data === true
}
