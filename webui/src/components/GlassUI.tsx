import React, { useState, useCallback, useContext, createContext } from 'react';

export function GlassCheck({ checked, indeterminate, onChange }: { checked: boolean; indeterminate?: boolean; onChange?: (val: boolean) => void }) {
  return (
    <div
      className={`gcheck ${checked ? 'checked' : ''} ${indeterminate && !checked ? 'indeterminate' : ''}`}
      onClick={(e) => { e.stopPropagation(); onChange?.(!checked); }}
      role="checkbox"
      aria-checked={indeterminate ? 'mixed' : checked}
    />
  );
}

export function StatusDot({ status }: { status: string }) {
  const color = status === 'running' ? 'ok' : status === 'stopped' ? 'off' : 'warn';
  return <span className={`dot ${color}`} title={status} />;
}

interface Toast {
  id: string;
  title: string;
  body?: string;
  kind?: 'success' | 'error' | 'info';
  progress?: number;
  auto?: boolean;
  duration?: number;
}

interface ToastContextType {
  push: (toast: Omit<Toast, 'id'>) => string;
  update: (id: string, patch: Partial<Omit<Toast, 'id'>>) => void;
  dismiss: (id: string) => void;
}

const ToastContext = createContext<ToastContextType | null>(null);

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const push = useCallback((toast: Omit<Toast, 'id'>) => {
    const id = Math.random().toString(36).slice(2);
    setToasts((prev) => [...prev, { id, ...toast }]);
    if (toast.auto !== false) {
      setTimeout(() => setToasts((prev) => prev.filter(x => x.id !== id)), toast.duration || 3500);
    }
    return id;
  }, []);

  const update = useCallback((id: string, patch: Partial<Omit<Toast, 'id'>>) => {
    setToasts((prev) => prev.map(t => t.id === id ? { ...t, ...patch } : t));
  }, []);

  const dismiss = useCallback((id: string) => setToasts(prev => prev.filter(t => t.id !== id)), []);

  return (
    <ToastContext.Provider value={{ push, update, dismiss }}>
      {children}
      <div className="toast-wrap">
        {toasts.map(t => (
          <div key={t.id} className="toast">
            <div className="row" style={{ justifyContent: 'space-between', alignItems: 'flex-start', gap: 10 }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>
                  {t.kind === 'success' && <span style={{ color: 'var(--ok)', marginRight: 6 }}>✓</span>}
                  {t.kind === 'error' && <span style={{ color: 'var(--danger)', marginRight: 6 }}>✕</span>}
                  {t.kind === 'info' && <span style={{ color: 'var(--accent-2)', marginRight: 6 }}>↻</span>}
                  {t.title}
                </div>
                {t.body && <div className="tiny muted" style={{ marginTop: 3 }}>{t.body}</div>}
                {typeof t.progress === 'number' && (
                  <div className="progress"><div style={{ width: `${t.progress}%` }} /></div>
                )}
              </div>
              <button className="btn ghost tiny" onClick={() => dismiss(t.id)} style={{ padding: '0 4px', fontSize: 16, color: 'var(--ink-3)' }}>×</button>
            </div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) throw new Error('useToast must be used within ToastProvider');
  return context;
}


export type SortOrder = { field: string; dir: 'asc' | 'desc' };

export function SortHeader({ label, field, sort, onSort, align = 'left', width }: {
  label: string; field: string; sort: SortOrder; onSort: (field: string) => void; align?: 'left' | 'center' | 'right'; width?: number | string
}) {
  const active = sort.field === field;
  return (
    <th
      className={`sortable ${active ? 'active-sort' : ''}`}
      style={{ textAlign: align, width }}
      onClick={() => onSort(field)}
    >
      {label}
      <span className="arrow">{active ? (sort.dir === 'asc' ? '↑' : '↓') : '↕'}</span>
    </th>
  );
}
