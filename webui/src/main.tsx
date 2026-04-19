import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { MantineProvider } from '@mantine/core'
import '@mantine/core/styles.css'
import { ToastProvider } from './components/GlassUI'
import App from './App.tsx'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <MantineProvider defaultColorScheme="dark">
      <ToastProvider>
        <App />
      </ToastProvider>
    </MantineProvider>
  </StrictMode>,
)
