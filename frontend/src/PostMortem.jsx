import { useEffect, useState } from 'react'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

export default function PostMortem({ gameId }) {
  const [report, setReport] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    if (!gameId) return
    fetch(`${API_URL}/reports/${gameId}`)
      .then((r) => r.json())
      .then(setReport)
      .catch((e) => setError(e.message))
  }, [gameId])

  if (error) return <div style={containerStyle}>Error: {error}</div>
  if (!report) return <div style={containerStyle}>Loading post-mortem...</div>

  return (
    <div style={containerStyle}>
      <h2 style={{ color: '#e2e8f0', marginBottom: 12 }}>Post-Mortem: {report.game_id.slice(0, 8)}</h2>
      <div style={{ color: '#94a3b8', marginBottom: 16, fontSize: 13 }}>
        {new Date(report.started_at).toLocaleTimeString()} → {new Date(report.ended_at).toLocaleTimeString()}
      </div>

      <div style={summaryStyle}>
        <Stat label="Total Attacks" value={report.summary.total_attacks} />
        <Stat label="Successful Heals" value={report.summary.successful_heals} color="#22c55e" />
        <Stat label="Failed Heals" value={report.summary.failed_heals} color="#ef4444" />
        <Stat label="Mean MTTR" value={`${Math.round(report.summary.mean_mttr_ms)}ms`} color="#f59e0b" />
        <Stat label="Services Affected" value={(report.summary.services_affected || []).join(', ')} />
      </div>

      <table style={tableStyle}>
        <thead>
          <tr>
            {['Service', 'Attack', 'Attacked At', 'MTTR (ms)', 'Outcome'].map((h) => (
              <th key={h} style={thStyle}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {(report.events || []).map((e, i) => (
            <tr key={i}>
              <td style={tdStyle}>{e.service}</td>
              <td style={tdStyle}>{e.attack}</td>
              <td style={tdStyle}>{new Date(e.attacked_at).toLocaleTimeString()}</td>
              <td style={tdStyle}>{e.mttr_ms ?? '—'}</td>
              <td style={{ ...tdStyle, color: e.outcome === 'healed' ? '#22c55e' : '#ef4444' }}>
                {e.outcome}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Stat({ label, value, color }) {
  return (
    <div style={{ marginRight: 24, marginBottom: 8 }}>
      <div style={{ color: '#64748b', fontSize: 11 }}>{label}</div>
      <div style={{ color: color || '#e2e8f0', fontSize: 18, fontWeight: 'bold' }}>{value}</div>
    </div>
  )
}

const containerStyle = {
  background: '#0f172a',
  border: '1px solid #1e293b',
  borderRadius: 8,
  padding: 16,
  color: '#e2e8f0',
  overflowY: 'auto',
  fontFamily: 'monospace',
}
const summaryStyle = { display: 'flex', flexWrap: 'wrap', marginBottom: 16 }
const tableStyle = { width: '100%', borderCollapse: 'collapse', fontSize: 12 }
const thStyle = { textAlign: 'left', padding: '6px 8px', borderBottom: '1px solid #1e293b', color: '#64748b' }
const tdStyle = { padding: '6px 8px', borderBottom: '1px solid #0f172a', color: '#e2e8f0' }
