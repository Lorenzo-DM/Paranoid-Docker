import { useState } from 'react'

import { useStacks } from './hooks/useStacks'
import { useContainers } from './hooks/useContainers'
import { useCapabilities } from './hooks/useCapabilities'
import { AppHeader } from './components/AppHeader'
import { StackTable } from './components/StackTable'
import { StackUpdateModal } from './components/StackUpdateModal'
import { StackLogsModal } from './components/StackLogsModal'
import { StackRollbacksModal } from './components/StackRollbacksModal'
import { BulkStackUpdateModal } from './components/BulkStackUpdateModal'
import { StackSaveModal, StackSaveUpdateModal } from './components/StackSaveModal'
import { ContainerTable } from './components/ContainerTable'
import { UpdateModal } from './components/UpdateModal'
import { LogsModal } from './components/LogsModal'
import { RollbacksModal } from './components/RollbacksModal'
import { SaveProgressModal } from './components/SaveProgressModal'
import { BulkContainerUpdateModal } from './components/BulkContainerUpdateModal'
import type { ContainerRef } from './components/BulkContainerUpdateModal'
import './App.css'

interface ContainerModal {
  id: string
  name: string
}

function App() {
  const { stacks, loading: stacksLoading, error: stacksError, refresh: refreshStacks } = useStacks()
  const { containers, loading: containersLoading, error: containersError, refresh: refreshContainers } = useContainers()
  const { capabilities, changeMode, refresh: refreshCaps } = useCapabilities()

  const refresh = () => { refreshStacks(); refreshContainers(); refreshCaps() }


  const [updatingStack, setUpdatingStack] = useState<string | null>(null)
  const [logsStack, setLogsStack] = useState<string | null>(null)
  const [rollbacksStack, setRollbacksStack] = useState<string | null>(null)
  const [bulkUpdatingStacks, setBulkUpdatingStacks] = useState<string[]>([])
  const [savingStack, setSavingStack] = useState<string | null>(null)
  const [saveUpdatingStack, setSaveUpdatingStack] = useState<string | null>(null)


  const [updatingContainer, setUpdatingContainer] = useState<ContainerModal | null>(null)
  const [logsContainer, setLogsContainer] = useState<ContainerModal | null>(null)
  const [rollbacksContainer, setRollbacksContainer] = useState<ContainerModal | null>(null)
  const [saveContainer, setSaveContainer] = useState<ContainerModal | null>(null)
  const [bulkUpdatingContainers, setBulkUpdatingContainers] = useState<ContainerRef[]>([])

  return (
    <div className="app-root">
      <AppHeader onRefresh={refresh} capabilities={capabilities} onModeChange={changeMode} />
      
      <main className="tbl-wrap">
        <div className="title-wrap">
          <h1>Compose stacks</h1>
          <span className="sub">{stacks.length} managed stacks</span>
        </div>
        
        <StackTable
          stacks={stacks}
          loading={stacksLoading}
          error={stacksError}
          onUpdate={name => setUpdatingStack(name)}
          onLogs={name => setLogsStack(name)}
          onRollbacks={name => setRollbacksStack(name)}
          onSave={name => setSavingStack(name)}
          onSaveAndUpdate={name => setSaveUpdatingStack(name)}
          onBulkUpdate={names => setBulkUpdatingStacks(names)}
        />

        {containers.length > 0 && (
          <>
            <div className="title-wrap standalone-title">
              <h1>Standalone containers</h1>
              <span className="sub">Not managed by compose</span>
            </div>
            
            <ContainerTable
              containers={containers}
              loading={containersLoading}
              error={containersError}
              onUpdate={(id, name) => setUpdatingContainer({ id, name })}
              onLogs={(id, name) => setLogsContainer({ id, name })}
              onRollbacks={(id, name) => setRollbacksContainer({ id, name })}
              onSave={(id, name) => setSaveContainer({ id, name })}
              onBulkUpdate={refs => setBulkUpdatingContainers(refs)}
            />
          </>
        )}
      </main>


      <StackUpdateModal
        stackName={updatingStack}
        onClose={() => { setUpdatingStack(null); refresh() }}
      />
      <StackLogsModal
        stackName={logsStack}
        onClose={() => setLogsStack(null)}
      />
      <StackRollbacksModal
        stackName={rollbacksStack}
        onClose={() => setRollbacksStack(null)}
      />
      <BulkStackUpdateModal
        stackNames={bulkUpdatingStacks}
        onClose={() => { setBulkUpdatingStacks([]); refresh() }}
      />
      <StackSaveModal
        stackName={savingStack}
        onClose={() => setSavingStack(null)}
      />
      <StackSaveUpdateModal
        stackName={saveUpdatingStack}
        onClose={() => { setSaveUpdatingStack(null); refresh() }}
      />


      <UpdateModal
        containerId={updatingContainer?.id ?? null}
        containerName={updatingContainer?.name ?? ''}
        onClose={() => { setUpdatingContainer(null); refresh() }}
      />
      <LogsModal
        containerId={logsContainer?.id ?? null}
        containerName={logsContainer?.name ?? ''}
        onClose={() => setLogsContainer(null)}
      />
      <RollbacksModal
        containerId={rollbacksContainer?.id ?? null}
        containerName={rollbacksContainer?.name ?? ''}
        onClose={() => setRollbacksContainer(null)}
      />
      <SaveProgressModal
        containerId={saveContainer?.id ?? null}
        containerName={saveContainer?.name ?? ''}
        onClose={() => setSaveContainer(null)}
      />
      <BulkContainerUpdateModal
        containers={bulkUpdatingContainers}
        onClose={() => { setBulkUpdatingContainers([]); refresh() }}
      />
    </div>
  )
}

export default App
