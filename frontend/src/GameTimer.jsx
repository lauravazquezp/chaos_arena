import { useState, useEffect } from 'react'

export default function GameTimer({ gameActive, startedAt, durationSeconds }) {
  const [remaining, setRemaining] = useState(null)

  useEffect(() => {
    if (!gameActive || !startedAt) {
      setRemaining(null)
      return
    }

    function update() {
      const elapsed = (Date.now() - new Date(startedAt).getTime()) / 1000
      const left = Math.max(0, durationSeconds - elapsed)
      setRemaining(left)
    }

    update()
    const id = setInterval(update, 250)
    return () => clearInterval(id)
  }, [gameActive, startedAt, durationSeconds])

  if (!gameActive || remaining === null) return null

  const pct     = remaining / durationSeconds
  const mins    = Math.floor(remaining / 60)
  const secs    = Math.floor(remaining % 60)
  const display = `${mins}:${String(secs).padStart(2, '0')}`
  const color   = pct > 0.4 ? '#22c55e' : pct > 0.15 ? '#f59e0b' : '#ef4444'

  return (
    <div style={{
      display:        'flex',
      alignItems:     'center',
      gap:            10,
      fontFamily:     'monospace',
    }}>
      <div style={{
        fontSize:   22,
        fontWeight: 'bold',
        color,
        minWidth:   48,
        textAlign:  'right',
      }}>
        {display}
      </div>
      {/* progress bar */}
      <div style={{
        width:        80,
        height:       6,
        background:   '#1e293b',
        borderRadius: 3,
        overflow:     'hidden',
      }}>
        <div style={{
          width:        `${pct * 100}%`,
          height:       '100%',
          background:   color,
          borderRadius: 3,
          transition:   'width 0.25s linear, background 0.5s',
        }} />
      </div>
    </div>
  )
}
