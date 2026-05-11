/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string
  readonly VITE_SHOW_ERROR_DETAIL?: string
  readonly VITE_ENV?: 'staging' | 'production'
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
