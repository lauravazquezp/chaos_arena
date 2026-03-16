import { useState, useEffect, useMemo } from 'react'
import Graph from './Graph.jsx'
import EventLog from './EventLog.jsx'
import PostMortem from './PostMortem.jsx'
import ConfigPanel from './ConfigPanel.jsx'
import { useWebSocket } from './useWebSocket.js'
import GameTimer from './GameTimer.jsx'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'
const WS_URL  = import.meta.env.VITE_WS_URL  || 'ws://localhost:8080/ws'

const ATTACK_TYPES = ['kill_switch', 'black_hole', 'resource_hog', 'latency_spike']

const IDLE = {}

export default function App() {
  const { services, events, gameId, gameOver, gameActive, startedAt, durationSeconds,
          simConfig, injectEvent, clearLog } = useWebSocket(WS_URL)
  const [gameRunning, setGameRunning] = useState(false)
  const [pending, setPending]         = useState(IDLE)
  const [localConfig, setLocalConfig] = useState(null) // fetched from REST on mount

  // Fetch initial config from REST so we have it before any WS event arrives.
  useEffect(() => {
    fetch(`${API_URL}/simulation/config`)
      .then(r => r.json())
      .then(cfg => setLocalConfig(cfg))
      .catch(() => {})
  }, [])

  // Use WS-updated config if available, fallback to REST-fetched.
  const config = simConfig || localConfig

  // Memoized so topology reference only changes when config changes,
  // preventing the D3 simulation from rebuilding on every event.
  const topology = useMemo(() => config
    ? config.services.map(id => ({
        id,
        depends_on: config.dependencies.filter(([from]) => from === id).map(([, to]) => to),
      }))
    : [], [config])

  const serviceIds = config?.services ?? []

  async function startGame() {
    const res = await fetch(`${API_URL}/game/start`, { method: 'POST' })
    if (res.ok) setGameRunning(true)
  }

  async function stopGame() {
    await fetch(`${API_URL}/game/stop`, { method: 'POST' })
    setGameRunning(false)
  }

  async function restartGame() {
    clearLog()
    if (gameRunning) await fetch(`${API_URL}/game/stop`, { method: 'POST' })
    const res = await fetch(`${API_URL}/game/start`, { method: 'POST' })
    if (res.ok) setGameRunning(true)
  }

  async function applyConfig(cfg) {
    const res = await fetch(`${API_URL}/simulation/config`, {
      method:  'POST',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify(cfg),
    })
    if (res.ok) setLocalConfig(cfg)
  }

  async function triggerAttack(service, attack) {
    const key = `${service}:${attack}`
    setPending((p) => ({ ...p, [key]: true }))
    injectEvent('attack.dispatched', { service, attack, local: true })
    try {
      await fetch(`${API_URL}/attack`, {
        method:  'POST',
        headers: { 'Content-Type': 'application/json' },
        body:    JSON.stringify({ service, attack }),
      })
    } finally {
      setPending((p) => { const n = { ...p }; delete n[key]; return n })
    }
  }

  const showPostMortem = gameOver && gameId

  return (
    <>
      <style>{`
        html, body, #root {
          margin: 0; padding: 0;
          background: #020617;
          height: 100%;
          overflow: hidden;
        }
      `}</style>
      <div style={styles.root}>
        {/* Graph: top 60% */}
        <div style={styles.graphArea}>
          <Graph services={services} topology={topology} gameActive={gameActive} />
          <div style={styles.graphTitle}>CHAOS ARENA</div>
          <div style={styles.timerOverlay}>
            <GameTimer gameActive={gameActive} startedAt={startedAt} durationSeconds={durationSeconds} onExpire={stopGame} />
          </div>
        </div>

        {/* Bottom row */}
        <div style={styles.bottom}>
          <div style={styles.eventLogArea}>
            <EventLog events={events} />
          </div>

          <div style={styles.controlsArea}>
            {showPostMortem ? (
              <>
                <PostMortem gameId={gameId} />
                <button style={{ ...styles.btn, background: '#22c55e', marginTop: 8, width: '100%' }} onClick={restartGame}>
                  New Simulation
                </button>
              </>
            ) : (
              <>
                {!gameRunning
                  ? <ConfigPanel config={config} onApply={applyConfig} disabled={false} />
                  : config?.heal_failure_probability > 0 && (
                      <div style={styles.healBadge}>
                        heal failure rate: <span style={{ color: config.heal_failure_probability > 0.4 ? '#ef4444' : config.heal_failure_probability > 0.15 ? '#f59e0b' : '#22c55e' }}>
                          {Math.round(config.heal_failure_probability * 100)}%
                        </span>
                      </div>
                    )
                }

                <div style={styles.controls}>
                  <div style={styles.gameButtons}>
                    <button
                      style={{ ...styles.btn, background: gameRunning ? '#475569' : '#22c55e' }}
                      onClick={startGame}
                      disabled={gameRunning}
                    >
                      Start
                    </button>
                    <button
                      style={{ ...styles.btn, background: '#3b82f6' }}
                      onClick={restartGame}
                    >
                      Restart
                    </button>
                    <button
                      style={{ ...styles.btn, background: gameRunning ? '#ef4444' : '#475569' }}
                      onClick={stopGame}
                      disabled={!gameRunning}
                    >
                      Stop
                    </button>
                  </div>

                  <div style={{ color: '#475569', fontSize: 11, marginBottom: 6 }}>ATTACK PANEL</div>
                  <div style={styles.attackGrid}>
                    {serviceIds.map((svc) =>
                      ATTACK_TYPES.map((atk) => {
                        const key        = `${svc}:${atk}`
                        const isPending  = !!pending[key]
                        const svcState   = services[svc]
                        const isActive   = svcState?.active_attack === atk
                        return (
                          <button
                            key={key}
                            style={{
                              ...styles.attackBtn,
                              background:  isPending ? '#92400e' : isActive ? '#7f1d1d' : '#1e293b',
                              borderColor: isPending ? '#f59e0b' : isActive ? '#ef4444' : '#334155',
                              opacity:     (!gameRunning || isPending) ? 0.5 : 1,
                            }}
                            onClick={() => triggerAttack(svc, atk)}
                            disabled={isPending || !gameRunning}
                          >
                            <span style={{ color: '#94a3b8', fontSize: 10 }}>{svc}</span>
                            <br />
                            <span style={{ fontSize: 10, color: isActive ? '#fca5a5' : '#e2e8f0' }}>
                              {isPending ? '…' : atk.replace(/_/g, ' ')}
                            </span>
                          </button>
                        )
                      })
                    )}
                  </div>
                </div>
              </>
            )}
          </div>
        </div>
      </div>
    </>
  )
}

const styles = {
  root: {
    display:       'flex',
    flexDirection: 'column',
    height:        '100vh',
    background:    '#020617',
    fontFamily:    'monospace',
  },
  graphArea: {
    flex:         '0 0 60%',
    position:     'relative',
    borderBottom: '1px solid #1e293b',
  },
  graphTitle: {
    position:      'absolute',
    top:           12,
    left:          16,
    color:         '#334155',
    fontSize:      11,
    letterSpacing: 4,
    fontWeight:    'bold',
  },
  timerOverlay: {
    position: 'absolute',
    top:      10,
    right:    16,
  },
  bottom: {
    flex:     1,
    display:  'flex',
    gap:      8,
    padding:  8,
    overflow: 'hidden',
  },
  eventLogArea: {
    flex:          '0 0 55%',
    overflow:      'hidden',
    minHeight:     0,
    display:       'flex',
    flexDirection: 'column',
  },
  controlsArea: {
    flex:      1,
    overflowY: 'auto',
    minHeight: 0,
  },
  controls: {
    background:   '#0f172a',
    border:       '1px solid #1e293b',
    borderRadius: 8,
    padding:      12,
  },
  gameButtons: {
    display:      'flex',
    gap:          8,
    marginBottom: 16,
  },
  btn: {
    padding:      '8px 14px',
    border:       'none',
    borderRadius: 6,
    color:        '#fff',
    cursor:       'pointer',
    fontFamily:   'monospace',
    fontWeight:   'bold',
    fontSize:     13,
  },
  healBadge: {
    background:   '#0f172a',
    border:       '1px solid #1e293b',
    borderRadius: 6,
    padding:      '6px 10px',
    fontSize:     11,
    color:        '#64748b',
    marginBottom: 8,
  },
  attackGrid: {
    display:             'grid',
    gridTemplateColumns: 'repeat(4, 1fr)',
    gap:                 6,
  },
  attackBtn: {
    padding:      '6px 4px',
    border:       '1px solid',
    borderRadius: 4,
    color:        '#e2e8f0',
    cursor:       'pointer',
    fontFamily:   'monospace',
    textAlign:    'center',
    lineHeight:   1.4,
    transition:   'background 0.15s, border-color 0.15s',
  },
}
