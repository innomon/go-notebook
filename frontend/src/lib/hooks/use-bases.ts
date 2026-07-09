import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { basesApi, WasmPluginResponse } from '@/lib/api/bases'
import { useToast } from '@/lib/hooks/use-toast'
import { useTranslation } from '@/lib/hooks/use-translation'
import { getApiErrorMessage } from '@/lib/utils/error-handler'

export const BASES_QUERY_KEYS = {
  plugins: ['bases', 'plugins'] as const,
}

export function useBasesPlugins() {
  return useQuery({
    queryKey: BASES_QUERY_KEYS.plugins,
    queryFn: () => basesApi.listPlugins(),
  })
}

export function useUpdatePluginPermissions() {
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: (data: WasmPluginResponse) => basesApi.updatePermissions(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BASES_QUERY_KEYS.plugins })
      toast({
        title: t('common.success'),
        description: t('bases.permissionsUpdateSuccess', { defaultValue: 'Permissions updated successfully.' }),
      })
    },
    onError: (error: unknown) => {
      toast({
        title: t('common.error'),
        description: getApiErrorMessage(error, (key) => t(key)),
        variant: 'destructive',
      })
    },
  })
}

export function useEvaluateBase() {
  const { toast } = useToast()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: ({ notebookId, config }: { notebookId: string; config: any }) =>
      basesApi.evaluate(notebookId, config),
    onError: (error: unknown) => {
      toast({
        title: t('common.error'),
        description: getApiErrorMessage(error, (key) => t(key)),
        variant: 'destructive',
      })
    },
  })
}
