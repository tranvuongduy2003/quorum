import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './app/styles.css'
import { Providers } from './app/providers.tsx'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Providers>
      <App />
    </Providers>
  </StrictMode>,
)
