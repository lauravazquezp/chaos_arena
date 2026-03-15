import { useEffect, useRef } from 'react'
import * as d3 from 'd3'

const STATUS_COLOR = {
  healthy:     '#22c55e',
  unhealthy:   '#ef4444',
  dying:       '#ef4444',
  provisioning:'transparent',
  unknown:     '#334155',
  latency:     '#f59e0b',
  inactive:    '#1e293b',
}

const RADIUS = 30

export default function Graph({ services, topology, gameActive }) {
  const svgRef = useRef(null)

  useEffect(() => {
    if (!topology || topology.length === 0) return

    const svg = d3.select(svgRef.current)
    svg.selectAll('*').remove()

    const width  = svgRef.current.clientWidth  || 800
    const height = svgRef.current.clientHeight || 400

    const nodes = topology.map((s) => ({ id: s.id, depends_on: s.depends_on }))
    const links = topology.flatMap((s) =>
      (s.depends_on || []).map((dep) => ({ source: s.id, target: dep }))
    )

    const sim = d3.forceSimulation(nodes)
      .force('link',   d3.forceLink(links).id((d) => d.id).distance(150))
      .force('charge', d3.forceManyBody().strength(-400))
      .force('center', d3.forceCenter(width / 2, height / 2))

    const defs = svg.append('defs')
    defs.append('marker')
      .attr('id', 'arrow')
      .attr('viewBox', '0 -5 10 10')
      .attr('refX', RADIUS + 10)
      .attr('refY', 0)
      .attr('markerWidth', 6)
      .attr('markerHeight', 6)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M0,-5L10,0L0,5')
      .attr('fill', '#334155')

    const link = svg.append('g')
      .selectAll('line')
      .data(links)
      .join('line')
      .attr('stroke', '#334155')
      .attr('stroke-width', 2)
      .attr('marker-end', 'url(#arrow)')

    const node = svg.append('g')
      .selectAll('g')
      .data(nodes)
      .join('g')
      .call(d3.drag()
        .on('start', (event, d) => { if (!event.active) sim.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y })
        .on('drag',  (event, d) => { d.fx = event.x; d.fy = event.y })
        .on('end',   (event, d) => { if (!event.active) sim.alphaTarget(0); d.fx = null; d.fy = null })
      )

    node.append('circle')
      .attr('r', RADIUS)
      .attr('fill',         (d) => nodeColor(d.id, services, gameActive))
      .attr('stroke',       (d) => nodeStroke(d.id, services, gameActive))
      .attr('stroke-width', 2)
      .attr('stroke-dasharray', (d) => {
        const svc = services[d.id]
        return (!gameActive || svc?.status === 'provisioning') ? '6,3' : 'none'
      })
      .attr('class', (d) => {
        const svc = services[d.id]
        return (gameActive && svc?.status === 'unhealthy') ? 'pulse' : ''
      })

    node.append('text')
      .attr('text-anchor', 'middle')
      .attr('dy', RADIUS + 16)
      .attr('fill', '#64748b')
      .attr('font-size', 12)
      .text((d) => d.id)

    sim.on('tick', () => {
      link
        .attr('x1', (d) => d.source.x).attr('y1', (d) => d.source.y)
        .attr('x2', (d) => d.target.x).attr('y2', (d) => d.target.y)
      node.attr('transform', (d) => `translate(${d.x},${d.y})`)
    })

    return () => sim.stop()
  }, [topology])

  // Re-color nodes when service state or gameActive changes without re-running simulation.
  useEffect(() => {
    if (!svgRef.current) return
    const svg = d3.select(svgRef.current)

    svg.selectAll('circle')
      .attr('fill',         function() { return nodeColor(d3.select(this.parentNode).datum()?.id, services, gameActive) })
      .attr('stroke',       function() { return nodeStroke(d3.select(this.parentNode).datum()?.id, services, gameActive) })
      .attr('stroke-dasharray', function() {
        const id  = d3.select(this.parentNode).datum()?.id
        const svc = services[id]
        return (!gameActive || svc?.status === 'provisioning') ? '6,3' : 'none'
      })
      .attr('class', function() {
        const id  = d3.select(this.parentNode).datum()?.id
        const svc = services[id]
        return (gameActive && svc?.status === 'unhealthy') ? 'pulse' : ''
      })
  }, [services, gameActive])

  return (
    <>
      <style>{`
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }
        .pulse { animation: pulse 1s ease-in-out infinite; }
      `}</style>
      <svg ref={svgRef} style={{ width: '100%', height: '100%', background: '#0f172a' }} />
    </>
  )
}

function nodeColor(id, services, gameActive) {
  if (!gameActive) return STATUS_COLOR.inactive
  const svc = services[id]
  if (!svc) return STATUS_COLOR.unknown
  if (svc.active_attack === 'latency_spike') return STATUS_COLOR.latency
  return STATUS_COLOR[svc.status] || STATUS_COLOR.unknown
}

function nodeStroke(id, services, gameActive) {
  if (!gameActive) return '#334155'
  const svc = services[id]
  return (svc?.status === 'provisioning') ? '#60a5fa' : 'none'
}
