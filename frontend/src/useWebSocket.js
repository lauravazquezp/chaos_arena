import { useEffect, useRef, useReducer, useCallback } from 'react'

const MAX_RETRIES = 5

function reducer(state, action) {
  if (action.type === '_reset') {
    return { ...initialState, services: state.services }
  }

  const event = { ...action, timestamp: action.timestamp || new Date().toISOString() }

  switch (action.type) {
    case 'container.dying':
      return {
        ...state,
        services: {
          ...state.services,
          [action.payload.service]: {
            ...state.services[action.payload.service],
            status: 'unhealthy',
            active_attack: action.payload.attack,
          },
        },
        events: [...state.events, event],
      }
    case 'container.provisioning':
      return {
        ...state,
        services: {
          ...state.services,
          [action.payload.service]: {
            ...state.services[action.payload.service],
            status: 'provisioning',
          },
        },
        events: [...state.events, event],
      }
    case 'container.healthy':
      return {
        ...state,
        services: {
          ...state.services,
          [action.payload.service]: {
            ...state.services[action.payload.service],
            status: 'healthy',
            active_attack: null,
          },
        },
        events: [...state.events, event],
      }
    case 'game.started': {
      // Reset service statuses to unknown so stale health state from a
      // previous game doesn't bleed into this one.
      const resetServices = {}
      for (const [id, svc] of Object.entries(state.services)) {
        resetServices[id] = { ...svc, status: 'unknown', active_attack: null }
      }
      return {
        ...state,
        services:        resetServices,
        gameId:          action.payload.game_id,
        gameOver:        false,
        gameActive:      true,
        startedAt:       action.payload.started_at,
        durationSeconds: action.payload.duration_seconds,
        events:          [...state.events, event],
      }
    }
    case 'game.ended':
      return { ...state, gameOver: true, gameActive: false, gameId: action.payload.game_id, events: [...state.events, event] }
    case 'reconciler.tick':
      // Only keep reconciler ticks while a game is running.
      if (!state.gameActive) return state
      return { ...state, events: [...state.events, event] }
    case 'simulation.config': {
      // Rebuild services map to match the new service list.
      const newServices = {}
      for (const id of (action.payload.services || [])) {
        newServices[id] = state.services[id] || { id, status: 'unknown', active_attack: null }
      }
      return { ...state, simConfig: action.payload, services: newServices }
    }
    default:
      return { ...state, events: [...state.events, event] }
  }
}

const initialState = {
  services: {}, events: [], gameId: null,
  gameOver: false, gameActive: false,
  startedAt: null, durationSeconds: 60,
  simConfig: null,
}

export function useWebSocket(url) {
  const [state, dispatch] = useReducer(reducer, initialState)
  const retriesRef = useRef(0)
  const wsRef = useRef(null)

  useEffect(() => {
    let cancelled = false

    function connect() {
      if (cancelled) return
      const ws = new WebSocket(url)
      wsRef.current = ws

      ws.onmessage = (e) => {
        try {
          const ev = JSON.parse(e.data)
          dispatch({ type: ev.type, payload: ev.payload })
        } catch {
          // ignore parse errors
        }
      }

      ws.onclose = () => {
        if (cancelled) return
        if (retriesRef.current < MAX_RETRIES) {
          const delay = Math.min(1000 * 2 ** retriesRef.current, 30000)
          retriesRef.current++
          setTimeout(connect, delay)
        }
      }

      ws.onopen = () => { retriesRef.current = 0 }
    }

    connect()
    return () => {
      cancelled = true
      wsRef.current?.close()
    }
  }, [url])

  const injectEvent = useCallback((type, payload) => {
    dispatch({ type, payload, timestamp: new Date().toISOString() })
  }, [])

  const clearLog = useCallback(() => {
    dispatch({ type: '_reset' })
  }, [])

  return { ...state, injectEvent, clearLog }
}
