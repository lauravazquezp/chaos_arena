import { useState, useEffect } from 'react'

const SERVICE_POOL = ['api-gateway', 'auth-service', 'payments', 'cache', 'worker']

function depKey(from, to) { return `${from}::${to}` }

function depsToMatrix(deps) {
  const m = {}
  for (const [from, to] of (deps || [])) m[depKey(from, to)] = true
  return m
}

export default function ConfigPanel({ config, onApply, disabled }) {
  const [count, setCount]     = useState(config?.services?.length ?? 3)
  const [matrix, setMatrix]   = useState(() => depsToMatrix(config?.dependencies))
  const [healPct, setHealPct] = useState(Math.round((config?.heal_failure_probability ?? 0) * 100))

  // Sync when config prop changes (e.g. fetched from backend).
  useEffect(() => {
    setCount(config?.services?.length ?? 3)
    setMatrix(depsToMatrix(config?.dependencies))
    setHealPct(Math.round((config?.heal_failure_probability ?? 0) * 100))
  }, [config])

  const services = SERVICE_POOL.slice(0, count)

  function toggle(from, to) {
    const k = depKey(from, to)
    setMatrix(m => ({ ...m, [k]: !m[k] }))
  }

  function handleApply() {
    const dependencies = []
    for (const from of services)
      for (const to of services)
        if (from !== to && matrix[depKey(from, to)])
          dependencies.push([from, to])
    onApply({ services, dependencies, heal_failure_probability: healPct / 100 })
  }

  // Short display name for labels in the matrix.
  const label = (id) => id.replace('api-gateway', 'api-gw').replace('auth-service', 'auth').replace('payments', 'pay')

  return (
    <div style={styles.panel}>
      <div style={styles.title}>CONFIGURE SIMULATION</div>

      {/* Service count */}
      <div style={styles.row}>
        <span style={styles.label}>Services</span>
        <div style={styles.countRow}>
          {[1, 2, 3, 4, 5].map(n => (
            <button
              key={n}
              style={{ ...styles.countBtn, background: count === n ? '#3b82f6' : '#1e293b', color: count === n ? '#fff' : '#64748b' }}
              onClick={() => setCount(n)}
              disabled={disabled}
            >{n}</button>
          ))}
        </div>
      </div>

      {/* Active service names */}
      <div style={styles.serviceNames}>
        {services.map(s => <span key={s} style={styles.tag}>{s}</span>)}
      </div>

      {/* Dependency matrix */}
      <div style={styles.label}>Dependencies</div>
      <div style={{ overflowX: 'auto' }}>
        <table style={styles.matrix}>
          <thead>
            <tr>
              <th style={styles.mhCorner}></th>
              {services.map(to => <th key={to} style={styles.mhCell}>{label(to)}</th>)}
            </tr>
          </thead>
          <tbody>
            {services.map(from => (
              <tr key={from}>
                <td style={styles.mhRow}>{label(from)}</td>
                {services.map(to => (
                  <td key={to} style={styles.mCell}>
                    {from === to
                      ? <span style={styles.dash}>—</span>
                      : <input
                          type="checkbox"
                          checked={!!matrix[depKey(from, to)]}
                          onChange={() => toggle(from, to)}
                          disabled={disabled}
                          style={{ cursor: 'pointer', accentColor: '#3b82f6' }}
                        />
                    }
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div style={styles.matrixHint}>row depends on column</div>

      {/* Heal failure probability */}
      <div style={{ ...styles.row, marginTop: 10 }}>
        <span style={styles.label}>Heal failure</span>
        <span style={{ ...styles.label, color: healPct > 40 ? '#ef4444' : healPct > 15 ? '#f59e0b' : '#22c55e' }}>{healPct}%</span>
      </div>
      <input
        type="range" min={0} max={80} step={5} value={healPct}
        onChange={e => setHealPct(Number(e.target.value))}
        disabled={disabled}
        style={{ width: '100%', accentColor: '#3b82f6', marginBottom: 10 }}
      />

      <button
        style={{ ...styles.applyBtn, opacity: disabled ? 0.4 : 1, cursor: disabled ? 'not-allowed' : 'pointer' }}
        onClick={handleApply}
        disabled={disabled}
      >
        Apply Config
      </button>
    </div>
  )
}

const styles = {
  panel: {
    background:   '#0f172a',
    border:       '1px solid #1e293b',
    borderRadius: 8,
    padding:      12,
    marginBottom: 8,
  },
  title: {
    color:         '#475569',
    fontSize:      10,
    letterSpacing: 2,
    marginBottom:  10,
  },
  row: {
    display:        'flex',
    justifyContent: 'space-between',
    alignItems:     'center',
    marginBottom:   6,
  },
  label: {
    color:    '#64748b',
    fontSize: 11,
  },
  countRow: {
    display: 'flex',
    gap:     4,
  },
  countBtn: {
    width:        24,
    height:       24,
    border:       'none',
    borderRadius: 4,
    fontFamily:   'monospace',
    fontSize:     12,
    cursor:       'pointer',
  },
  serviceNames: {
    display:      'flex',
    gap:          4,
    flexWrap:     'wrap',
    marginBottom: 10,
  },
  tag: {
    background:   '#1e293b',
    color:        '#94a3b8',
    fontSize:     10,
    padding:      '2px 6px',
    borderRadius: 3,
  },
  matrix: {
    borderCollapse: 'collapse',
    fontSize:       10,
    marginBottom:   2,
    width:          '100%',
  },
  mhCorner: {
    width: 40,
  },
  mhCell: {
    color:      '#475569',
    fontWeight: 'normal',
    textAlign:  'center',
    padding:    '2px 4px',
  },
  mhRow: {
    color:     '#475569',
    textAlign: 'right',
    padding:   '2px 6px 2px 0',
  },
  mCell: {
    textAlign: 'center',
    padding:   '3px 4px',
  },
  dash: {
    color: '#1e293b',
  },
  matrixHint: {
    color:        '#334155',
    fontSize:     9,
    marginBottom: 4,
  },
  applyBtn: {
    width:        '100%',
    padding:      '7px 0',
    background:   '#1e40af',
    border:       'none',
    borderRadius: 6,
    color:        '#fff',
    fontFamily:   'monospace',
    fontSize:     12,
    fontWeight:   'bold',
  },
}
