<template>
  <div class="topo-container" ref="containerRef">
    <svg
      ref="svgRef"
      :width="svgWidth"
      :height="svgHeight"
      @mousemove="onMouseMove"
      @mouseup="onMouseUp"
      @mouseleave="onMouseUp"
    >
      <defs>
        <marker
          id="arrow-gray"
          markerWidth="10"
          markerHeight="7"
          refX="10"
          refY="3.5"
          orient="auto"
        >
          <polygon points="0 0, 10 3.5, 0 7" fill="#c0c4cc" />
        </marker>
        <marker
          id="arrow-red"
          markerWidth="10"
          markerHeight="7"
          refX="10"
          refY="3.5"
          orient="auto"
        >
          <polygon points="0 0, 10 3.5, 0 7" fill="#f56c6c" />
        </marker>
        <marker
          id="arrow-orange"
          markerWidth="10"
          markerHeight="7"
          refX="10"
          refY="3.5"
          orient="auto"
        >
          <polygon points="0 0, 10 3.5, 0 7" fill="#e6a23c" />
        </marker>
        <marker
          id="arrow-highlight"
          markerWidth="10"
          markerHeight="7"
          refX="10"
          refY="3.5"
          orient="auto"
        >
          <polygon points="0 0, 10 3.5, 0 7" fill="#409eff" />
        </marker>
      </defs>

      <g v-for="edge in edgeList" :key="edge.id">
        <line
          :x1="edge.x1"
          :y1="edge.y1"
          :x2="edge.x2"
          :y2="edge.y2"
          :stroke="getEdgeColor(edge)"
          :stroke-width="edge.highlighted ? 3 : 1.5"
          :marker-end="getEdgeMarker(edge)"
          :class="{ 'edge-highlighted': edge.highlighted }"
          @mouseenter="showEdgeTooltip($event, edge)"
          @mouseleave="hideEdgeTooltip"
        />
      </g>

      <g
        v-for="node in nodeList"
        :key="node.id"
        :transform="`translate(${node.x}, ${node.y})`"
        @mousedown.prevent="onNodeMouseDown($event, node)"
        @click.stop="onNodeClick(node)"
        class="topo-node"
        :class="{ 'node-highlighted': node.highlighted, 'node-selected': selectedNodeId === node.id }"
      >
        <rect
          :x="-nodeWidth / 2"
          :y="-nodeHeight / 2"
          :width="nodeWidth"
          :height="nodeHeight"
          rx="8"
          :fill="(node.highlighted || selectedNodeId === node.id) ? '#ecf5ff' : '#fff'"
          :stroke="getNodeStroke(node)"
          :stroke-width="(selectedNodeId === node.id) ? 2.5 : (node.highlighted ? 2 : 1)"
        />
        <text
          :x="0"
          :y="-4"
          text-anchor="middle"
          :fill="methodColors[node.method] || '#606266'"
          font-size="11"
          font-weight="700"
          font-family="SF Mono, Fira Code, monospace"
        >{{ node.method }}</text>
        <text
          :x="0"
          :y="12"
          text-anchor="middle"
          fill="#606266"
          font-size="11"
          font-family="SF Mono, Fira Code, monospace"
        >{{ truncatePath(node.path) }}</text>
      </g>
    </svg>

    <div
      v-if="tooltip.visible"
      class="edge-tooltip"
      :style="{ left: tooltip.x + 'px', top: tooltip.y + 'px' }"
    >
      <div class="tooltip-title">字段映射</div>
      <div v-for="(m, i) in tooltip.mappings" :key="i" class="tooltip-mapping">
        <span class="tooltip-from">{{ m.upstreamField }}</span>
        <span class="tooltip-arrow">→</span>
        <span class="tooltip-to">{{ m.downstreamField }}</span>
      </div>
    </div>

    <div v-if="nodeList.length === 0" class="topo-empty">
      暂无依赖关系，无法生成拓扑图
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'

const props = defineProps({
  projectId: { type: String, default: '' },
  dependencies: { type: Array, default: () => [] },
  apis: { type: Array, default: () => [] },
  impactReports: { type: Array, default: () => [] }
})

const containerRef = ref(null)
const svgRef = ref(null)
const svgWidth = ref(900)
const svgHeight = ref(500)
const nodeWidth = 120
const nodeHeight = 40
const selectedNodeId = ref(null)
const highlightedNodeIds = reactive(new Set())
const highlightedEdgeIds = reactive(new Set())

const nodeMap = reactive(new Map())
const nodeList = ref([])
const edgeList = ref([])

const dragging = ref(null)
const dragOffset = ref({ x: 0, y: 0 })
const hasDragged = ref(false)

const tooltip = ref({ visible: false, x: 0, y: 0, mappings: [] })

const methodColors = {
  GET: '#67c23a',
  POST: '#409eff',
  PUT: '#e6a23c',
  PATCH: '#909399',
  DELETE: '#f56c6c',
  HEAD: '#909399',
  OPTIONS: '#909399'
}

const storageKey = computed(() => {
  return 'topo-positions-' + props.projectId
})

function loadPositions() {
  try {
    const raw = localStorage.getItem(storageKey.value)
    if (raw) return JSON.parse(raw)
  } catch {}
  return null
}

function savePositions() {
  const pos = {}
  nodeList.value.forEach(n => { pos[n.id] = { x: n.x, y: n.y } })
  try {
    localStorage.setItem(storageKey.value, JSON.stringify(pos))
    console.log('[Topo] Saved positions to localStorage:', pos)
  } catch (e) {
    console.error('[Topo] Failed to save positions:', e)
  }
}

const edgeImpactMap = computed(() => {
  const m = new Map()
  for (const report of props.impactReports) {
    const affected = parseAffected(report.affected_downstream)
    for (const ad of affected) {
      const key = `${report.changed_api_id}->${ad.downstream_api_id}`
      if (!m.has(key) || ad.impact_level === 'Breaking') {
        m.set(key, ad.impact_level)
      }
    }
  }
  return m
})

function parseAffected(affected) {
  if (!affected) return []
  if (typeof affected === 'string') {
    try { return JSON.parse(affected) } catch { return [] }
  }
  return affected
}

function rebuildNodeList() {
  const savedPos = loadPositions()
  const newNodeMap = new Map()

  for (const api of props.apis) {
    if (newNodeMap.has(api.id)) continue
    const existing = nodeMap.get(api.id)
    if (existing) {
      newNodeMap.set(api.id, existing)
    } else {
      newNodeMap.set(api.id, {
        id: api.id,
        method: api.method,
        path: api.path,
        x: 0, y: 0,
        highlighted: false
      })
    }
  }

  for (const dep of props.dependencies) {
    for (const [apiId, method, path] of [
      [dep.upstream_api_id, dep.upstream_method, dep.upstream_path],
      [dep.downstream_api_id, dep.downstream_method, dep.downstream_path]
    ]) {
      if (!newNodeMap.has(apiId)) {
        const existing = nodeMap.get(apiId)
        if (existing) {
          newNodeMap.set(apiId, existing)
        } else {
          newNodeMap.set(apiId, {
            id: apiId,
            method,
            path,
            x: 0, y: 0,
            highlighted: false
          })
        }
      }
    }
  }

  const allNodes = Array.from(newNodeMap.values())

  if (savedPos) {
    for (const n of allNodes) {
      if (savedPos[n.id]) {
        n.x = savedPos[n.id].x
        n.y = savedPos[n.id].y
      }
    }
  }

  const hasAnyPosition = allNodes.some(n => n.x !== 0 || n.y !== 0)

  if (!hasAnyPosition) {
    const upstreamSet = new Set()
    const downstreamSet = new Set()
    for (const dep of props.dependencies) {
      upstreamSet.add(dep.upstream_api_id)
      downstreamSet.add(dep.downstream_api_id)
    }

    const layers = []
    const assigned = new Set()
    let currentLayer = allNodes.filter(n => upstreamSet.has(n.id) && !downstreamSet.has(n.id))
    if (currentLayer.length === 0) {
      currentLayer = allNodes.filter(n => upstreamSet.has(n.id))
    }
    if (currentLayer.length === 0 && allNodes.length > 0) {
      currentLayer = [allNodes[0]]
    }

    while (assigned.size < allNodes.length && currentLayer.length > 0) {
      layers.push([...currentLayer])
      currentLayer.forEach(n => assigned.add(n.id))
      const nextLayer = []
      for (const node of currentLayer) {
        for (const dep of props.dependencies) {
          if (dep.upstream_api_id === node.id && !assigned.has(dep.downstream_api_id)) {
            const n = allNodes.find(a => a.id === dep.downstream_api_id)
            if (n && !nextLayer.find(x => x.id === n.id)) {
              nextLayer.push(n)
            }
          }
        }
      }
      currentLayer = nextLayer
    }

    const remaining = allNodes.filter(n => !assigned.has(n.id))
    if (remaining.length > 0) {
      layers.push(remaining)
    }

    const layerGap = 200
    const nodeGap = 80
    const startX = 100

    for (let li = 0; li < layers.length; li++) {
      const layer = layers[li]
      const totalH = (layer.length - 1) * nodeGap
      const offsetY = (svgHeight.value - totalH) / 2
      for (let ni = 0; ni < layer.length; ni++) {
        const n = allNodes.find(a => a.id === layer[ni].id)
        if (n && !(savedPos && savedPos[n.id])) {
          n.x = startX + li * layerGap
          n.y = Math.max(nodeHeight, offsetY + ni * nodeGap)
        }
      }
    }
  }

  nodeMap.clear()
  allNodes.forEach(n => nodeMap.set(n.id, n))
  nodeList.value = allNodes

  rebuildEdgeList()
}

function rebuildEdgeList() {
  const edges = props.dependencies.map(dep => {
    const src = nodeMap.get(dep.upstream_api_id)
    const tgt = nodeMap.get(dep.downstream_api_id)
    if (!src || !tgt) return null

    const dx = tgt.x - src.x
    const dy = tgt.y - src.y
    const len = Math.sqrt(dx * dx + dy * dy) || 1

    const x1 = src.x + (dx / len) * (nodeWidth / 2 + 2)
    const y1 = src.y + (dy / len) * (nodeHeight / 2 + 2)
    const x2 = tgt.x - (dx / len) * (nodeWidth / 2 + 12)
    const y2 = tgt.y - (dy / len) * (nodeHeight / 2 + 2)

    return {
      id: dep.id,
      upstreamId: dep.upstream_api_id,
      downstreamId: dep.downstream_api_id,
      impactKey: `${dep.upstream_api_id}->${dep.downstream_api_id}`,
      x1, y1, x2, y2,
      fieldMappings: dep.field_mappings,
      highlighted: highlightedEdgeIds.has(dep.id)
    }
  }).filter(Boolean)

  edgeList.value = edges
}

function getEdgeColor(edge) {
  if (edge.highlighted) return '#409eff'
  const level = edgeImpactMap.value.get(edge.impactKey)
  if (level === 'Breaking') return '#f56c6c'
  if (level === 'Warning') return '#e6a23c'
  return '#c0c4cc'
}

function getEdgeMarker(edge) {
  if (edge.highlighted) return 'url(#arrow-highlight)'
  const level = edgeImpactMap.value.get(edge.impactKey)
  if (level === 'Breaking') return 'url(#arrow-red)'
  if (level === 'Warning') return 'url(#arrow-orange)'
  return 'url(#arrow-gray)'
}

function getNodeStroke(node) {
  if (selectedNodeId.value === node.id) return '#409eff'
  if (node.highlighted) return '#409eff'
  return '#dcdfe6'
}

function truncatePath(path) {
  if (!path) return ''
  if (path.length > 16) return '...' + path.slice(-13)
  return path
}

function onNodeMouseDown(e, node) {
  dragging.value = node
  hasDragged.value = false
  const svgRect = svgRef.value.getBoundingClientRect()
  dragOffset.value = {
    x: e.clientX - svgRect.left - node.x,
    y: e.clientY - svgRect.top - node.y
  }
  console.log('[Topo] Drag start:', node.id, node.x, node.y)
}

function onMouseMove(e) {
  if (!dragging.value) return
  const svgRect = svgRef.value.getBoundingClientRect()
  const newX = e.clientX - svgRect.left - dragOffset.value.x
  const newY = e.clientY - svgRect.top - dragOffset.value.y
  if (Math.abs(newX - dragging.value.x) > 1 || Math.abs(newY - dragging.value.y) > 1) {
    hasDragged.value = true
  }
  dragging.value.x = newX
  dragging.value.y = newY
  rebuildEdgeList()
}

function onMouseUp() {
  if (dragging.value) {
    console.log('[Topo] Drag end:', dragging.value.id, dragging.value.x, dragging.value.y)
    const nodeId = dragging.value.id
    dragging.value = null
    if (hasDragged.value) {
      savePositions()
    }
    setTimeout(() => { hasDragged.value = false }, 50)
  }
}

function onNodeClick(node) {
  if (hasDragged.value) return

  console.log('[Topo] Node click:', node.id)

  if (selectedNodeId.value === node.id) {
    selectedNodeId.value = null
    highlightedNodeIds.clear()
    highlightedEdgeIds.clear()
    nodeList.value.forEach(n => { n.highlighted = false })
    edgeList.value.forEach(e => { e.highlighted = false })
    return
  }

  selectedNodeId.value = node.id
  highlightedNodeIds.clear()
  highlightedEdgeIds.clear()

  const traceUp = (id, visited = new Set()) => {
    if (visited.has(id)) return
    visited.add(id)
    for (const dep of props.dependencies) {
      if (dep.downstream_api_id === id) {
        highlightedNodeIds.add(dep.upstream_api_id)
        highlightedEdgeIds.add(dep.id)
        traceUp(dep.upstream_api_id, visited)
      }
    }
  }

  const traceDown = (id, visited = new Set()) => {
    if (visited.has(id)) return
    visited.add(id)
    for (const dep of props.dependencies) {
      if (dep.upstream_api_id === id) {
        highlightedNodeIds.add(dep.downstream_api_id)
        highlightedEdgeIds.add(dep.id)
        traceDown(dep.downstream_api_id, visited)
      }
    }
  }

  highlightedNodeIds.add(node.id)
  traceUp(node.id)
  traceDown(node.id)

  console.log('[Topo] Highlighted nodes:', Array.from(highlightedNodeIds))
  console.log('[Topo] Highlighted edges:', Array.from(highlightedEdgeIds))

  nodeList.value.forEach(n => {
    n.highlighted = highlightedNodeIds.has(n.id)
  })
  edgeList.value.forEach(e => {
    e.highlighted = highlightedEdgeIds.has(e.id)
  })
}

function showEdgeTooltip(e, edge) {
  const mappings = parseMappings(edge.fieldMappings)
  const containerRect = containerRef.value.getBoundingClientRect()
  tooltip.value = {
    visible: true,
    x: e.clientX - containerRect.left + 12,
    y: e.clientY - containerRect.top + 12,
    mappings
  }
}

function hideEdgeTooltip() {
  tooltip.value.visible = false
}

function parseMappings(mappings) {
  if (!mappings) return []
  let parsed = mappings
  if (typeof mappings === 'string') {
    try { parsed = JSON.parse(mappings) } catch { return [] }
  }
  if (!Array.isArray(parsed)) return []
  return parsed.map(m => ({
    upstreamField: m.upstreamField || m.upstream_field || '',
    downstreamField: m.downstreamField || m.downstream_field || ''
  }))
}

function updateSize() {
  if (containerRef.value) {
    svgWidth.value = Math.max(600, containerRef.value.clientWidth - 2)
    svgHeight.value = Math.max(400, Math.min(600, nodeList.value.length * 50 + 100))
  }
}

let rebuildTimer = null
function scheduleRebuild() {
  if (rebuildTimer) clearTimeout(rebuildTimer)
  rebuildTimer = setTimeout(() => {
    rebuildNodeList()
    nextTick(updateSize)
  }, 50)
}

watch(() => [props.dependencies, props.apis], () => {
  scheduleRebuild()
}, { deep: true })

onMounted(() => {
  scheduleRebuild()
  window.addEventListener('resize', updateSize)
})

onUnmounted(() => {
  window.removeEventListener('resize', updateSize)
  if (rebuildTimer) clearTimeout(rebuildTimer)
})
</script>

<style scoped>
.topo-container {
  position: relative;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  overflow: hidden;
  background: #fafafa;
  margin-bottom: 20px;
}

.topo-node {
  cursor: grab;
}

.topo-node:active {
  cursor: grabbing;
}

.topo-node.node-highlighted rect {
  filter: drop-shadow(0 0 4px rgba(64, 158, 255, 0.4));
}

.topo-node.node-selected rect {
  filter: drop-shadow(0 0 6px rgba(64, 158, 255, 0.6));
}

.edge-highlighted {
  filter: drop-shadow(0 0 3px rgba(64, 158, 255, 0.5));
}

.edge-tooltip {
  position: absolute;
  background: #303133;
  color: #fff;
  border-radius: 6px;
  padding: 10px 14px;
  font-size: 12px;
  pointer-events: none;
  z-index: 100;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  max-width: 280px;
}

.tooltip-title {
  font-weight: 600;
  margin-bottom: 6px;
  border-bottom: 1px solid #606266;
  padding-bottom: 4px;
}

.tooltip-mapping {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 0;
}

.tooltip-from {
  color: #67c23a;
  font-family: SF Mono, Fira Code, monospace;
}

.tooltip-arrow {
  color: #909399;
}

.tooltip-to {
  color: #409eff;
  font-family: SF Mono, Fira Code, monospace;
}

.topo-empty {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: #909399;
  font-size: 14px;
}
</style>
