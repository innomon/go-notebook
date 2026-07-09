import apiClient from './client'
import { A2UIResponse } from '@/components/bases/A2UIRenderer'

export interface WasmPluginResponse {
  name: string
  read_other_notes: boolean
  access_env: boolean
}

export const basesApi = {
  listPlugins: async () => {
    const response = await apiClient.get<WasmPluginResponse[]>('/bases/plugins')
    return response.data
  },

  updatePermissions: async (data: WasmPluginResponse) => {
    const response = await apiClient.post<{ status: string }>('/bases/plugins/permissions', data)
    return response.data
  },

  evaluate: async (notebookId: string, config: any) => {
    const response = await apiClient.post<A2UIResponse>('/bases/evaluate', {
      notebook_id: notebookId || undefined,
      config,
    })
    return response.data
  },
}
