import type { ComposeService } from '../types/api'
import { UpdateBadge } from './UpdateBadge'
import { StatusDot } from './GlassUI'

interface Props {
  services: ComposeService[]
}

export function ServicesTable({ services }: Props) {
  return (
    <div className="tbl-shell" style={{ margin: 0, border: 'none', background: 'transparent', boxShadow: 'none' }}>
      <table className="wtable" style={{ fontSize: '12px' }}>
        <thead>
          <tr>
            <th style={{ background: 'transparent' }}>Service</th>
            <th style={{ background: 'transparent' }}>Image</th>
            <th style={{ background: 'transparent' }}>State</th>
            <th style={{ background: 'transparent' }}>Ports</th>
          </tr>
        </thead>
        <tbody>
          {services.map(svc => {
            const ports = svc.ports
              .filter(p => p.host_port !== '0')
              .map(p => `${p.host_port}:${p.container_port}`)
              .join(', ')
            return (
              <tr key={svc.name}>
                <td><span className="mono">{svc.name}</span></td>
                <td><span className="mono muted tiny">{svc.image}</span></td>
                <td>
                  <div className="row">
                    <StatusDot status={svc.state === 'running' ? 'running' : 'stopped'} />
                    <UpdateBadge updateAvailable={svc.update_available} />
                  </div>
                </td>
                <td className="muted tiny">{ports || '—'}</td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
