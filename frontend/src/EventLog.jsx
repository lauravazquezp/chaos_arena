import { useRef, useEffect } from 'react'

const MAX_EVENTS = 100

const EVENT_LABELS = {
  'container.dying':        'attack active',
  'container.provisioning': 'healing: running undo',
  'container.healthy':      'recovered',
  'heal.failed':            'heal attempt failed — retrying',
  'control_plane.error':    'control-plane error',
  'game.started':           'simulation started',
  'game.ended':             'simulation ended',
  'attack.dispatched':      'attack dispatched (pending)',
}

export default function EventLog({ events }) {
  const bottomRef = useRef(null)
  const displayed = events.slice(-MAX_EVENTS)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [events])

  return (
    <div style={{
      background: '#0f172a',
      border: '1px solid #1e293b',
      borderRadius: 8,
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      fontFamily: 'monospace',
      fontSize: 12,
      color: '#94a3b8',
      overflow: 'hidden',
    }}>
      <div style={{ fontWeight: 'bold', color: '#e2e8f0', padding: '10px 12px 6px', flexShrink: 0 }}>Event Log</div>
      <div style={{ flex: 1, overflowY: 'auto', padding: '0 12px 12px' }}>
        {displayed.length === 0 && (
          <div style={{ color: '#334155' }}>Waiting for events…</div>
        )}
        {displayed.map((e, i) => {
          if (e.type === 'reconciler.tick') return <ReconcilerTick key={i} event={e} />
          return <RegularEvent key={i} event={e} />
        })}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}

function ReconcilerTick({ event }) {
  const findings = Array.isArray(event.payload) ? event.payload : []
  const allHealthy = findings.length > 0 && findings.every((f) => f.healthy && !f.attack)
  return (
    <div style={{ marginBottom: 6, borderLeft: '2px solid #1e3a5f', paddingLeft: 8 }}>
      <div style={{ color: '#334155' }}>
        <span style={{ color: '#1e3a5f' }}>[{(event.timestamp || '').slice(11, 23)}]</span>
        {' '}
        <span style={{ color: '#2563eb' }}>reconciler tick</span>
        {allHealthy && <span style={{ color: '#166534' }}> — all services healthy</span>}
      </div>
      {!allHealthy && findings.map((f) => (
        <div key={f.service} style={{ paddingLeft: 12, marginTop: 2 }}>
          <span style={{ color: '#475569' }}>{f.service}</span>
          {' '}
          <span style={{ color: f.healthy ? '#166534' : '#7f1d1d' }}>
            {f.healthy ? 'healthy' : 'unhealthy'}
          </span>
          {' '}
          <span style={{ color: '#334155' }}>({f.status})</span>
          {f.attack && <span style={{ color: '#78350f' }}> [{f.attack.replace(/_/g, ' ')}]</span>}
        </div>
      ))}
    </div>
  )
}

function RegularEvent({ event: e }) {
  return (
    <div style={{ marginBottom: 6, opacity: e.local ? 0.65 : 1 }}>
      <div style={{ color: eventColor(e.type) }}>
        <span style={{ color: '#475569' }}>[{(e.timestamp || '').slice(11, 23)}]</span>{' '}
        <span style={{ fontWeight: 'bold' }}>{EVENT_LABELS[e.type] || e.type}</span>
        {e.payload?.service && <span style={{ color: '#cbd5e1' }}> — {e.payload.service}</span>}
        {e.payload?.attack  && <span style={{ color: '#f59e0b' }}> ({e.payload.attack.replace(/_/g, ' ')})</span>}
        {e.payload?.mttr_ms != null && e.payload.mttr_ms > 0 && (
          <span style={{ color: '#22c55e' }}> MTTR: {e.payload.mttr_ms}ms</span>
        )}
      </div>
      {e.payload?.detail && (
        <div style={{ color: '#475569', paddingLeft: 68, marginTop: 1 }}>{e.payload.detail}</div>
      )}
      {e.payload?.error && (
        <div style={{ color: '#ef4444', paddingLeft: 68, marginTop: 1 }}>{e.payload.error}</div>
      )}
    </div>
  )
}

function eventColor(type) {
  switch (type) {
    case 'container.dying':        return '#ef4444'
    case 'container.healthy':      return '#22c55e'
    case 'container.provisioning': return '#60a5fa'
    case 'control_plane.error':    return '#f97316'
    case 'game.started':           return '#3b82f6'
    case 'game.ended':             return '#8b5cf6'
    case 'attack.dispatched':      return '#f59e0b'
    default:                       return '#94a3b8'
  }
}
