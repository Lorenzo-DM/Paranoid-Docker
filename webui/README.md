# webui

React frontend for update-docker-container.

## Stack

- React 19 + TypeScript
- Vite 8
- Mantine 9 (UI components)
- Tabler Icons
- xterm.js (log terminals)

## Structure

```
src/
  api/          # API client functions (fetch wrappers)
  components/   # UI components (tables, modals)
  hooks/        # Data-fetching hooks (useStacks, useContainers)
  types/        # Shared TypeScript types
  settings.ts   # API base URL and config
  App.tsx       # Root component
```

## Setup

```bash
bun install
bun run dev
```

Dev server runs on `http://localhost:5173`. Expects backend on `http://localhost:1323`.

## Build

```bash
bun run build
```

Output in `dist/`.

## Configuration

Edit `src/settings.ts` to change the API base URL.
