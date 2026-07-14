import React, { useState, useCallback, useContext, createContext } from 'react';
import { Progress } from '@mantine/core';

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
            <div className="row toast-row">
              <div className="toast-body">
                <div className="toast-title">
                  {t.kind === 'success' && <span className="toast-icon toast-icon-ok">✓</span>}
                  {t.kind === 'error' && <span className="toast-icon toast-icon-danger">✕</span>}
                  {t.kind === 'info' && <span className="toast-icon toast-icon-info">↻</span>}
                  {t.title}
                </div>
                {t.body && <div className="tiny muted toast-body-text">{t.body}</div>}
                {typeof t.progress === 'number' && (
                  <Progress value={t.progress} size="xs" mt={8} />
                )}
              </div>
              <button className="btn ghost tiny toast-dismiss" onClick={() => dismiss(t.id)}>×</button>
            </div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components -- hook lives beside its provider
export function useToast() {
  const context = useContext(ToastContext);
  if (!context) throw new Error('useToast must be used within ToastProvider');
  return context;
}


export type SortOrder = { field: string; dir: 'asc' | 'desc' };

export function SortHeader({ label, field, sort, onSort, align = 'left', className }: {
  label: string; field: string; sort: SortOrder; onSort: (field: string) => void; align?: 'left' | 'center' | 'right'; className?: string
}) {
  const active = sort.field === field;
  const alignCls = align !== 'left' ? `text-${align}` : '';
  return (
    <th
      className={`sortable ${active ? 'active-sort' : ''} ${alignCls} ${className ?? ''}`}
      onClick={() => onSort(field)}
    >
      {label}
      <span className="arrow">{active ? (sort.dir === 'asc' ? '↑' : '↓') : '↕'}</span>
    </th>
  );
}
