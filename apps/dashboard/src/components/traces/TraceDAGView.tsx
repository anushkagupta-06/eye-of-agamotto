import React, { useMemo } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  MarkerType,
  type Node,
  type Edge,
  type NodeProps,
  Handle,
  Position,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Zap, AlertCircle, CheckCircle2 } from 'lucide-react';
import type { Span } from '../../types/telemetry';

export interface TraceDAGViewProps {
  spans: Span[];
  selectedSpanId?: string;
  onSelectSpan: (span: Span) => void;
  getServiceColor: (service: string, isError: boolean) => string;
}

interface SpanNodeData {
  span: Span;
  durationMs: number;
  isCritical: boolean;
  isSelected: boolean;
  serviceColor: string;
}

// Custom Node Component for Distributed Spans in the DAG
const CustomSpanNode: React.FC<NodeProps> = ({ data }) => {
  const nodeData = data as unknown as SpanNodeData;
  const { span, durationMs, isCritical, isSelected, serviceColor } = nodeData;
  const isError = span.statusCode === 'ERROR';

  return (
    <div
      style={{
        width: '230px',
        borderRadius: '10px',
        backgroundColor: 'var(--bg-card)',
        border: isSelected
          ? '2px solid var(--accent-emerald)'
          : isCritical
          ? '2px solid #f59e0b'
          : isError
          ? '2px solid #ef4444'
          : '1px solid var(--border-subtle)',
        boxShadow: isSelected
          ? '0 0 16px rgba(16, 185, 129, 0.4)'
          : isCritical
          ? '0 0 14px rgba(245, 158, 11, 0.35)'
          : isError
          ? '0 0 12px rgba(239, 68, 68, 0.3)'
          : '0 4px 12px rgba(0, 0, 0, 0.3)',
        padding: '10px 12px',
        fontSize: '0.75rem',
        cursor: 'pointer',
        position: 'relative',
        transition: 'all 0.15s ease',
      }}
    >
      {/* Input Handle (Incoming call) */}
      <Handle
        type="target"
        position={Position.Top}
        style={{ background: isCritical ? '#f59e0b' : 'var(--text-muted)', width: 8, height: 8 }}
      />

      {/* Header: Service Name & Indicator */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '6px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <span
            style={{
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              backgroundColor: serviceColor,
            }}
          />
          <strong
            style={{
              color: isError ? '#f87171' : '#fff',
              fontSize: '0.8rem',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              maxWidth: '120px',
            }}
          >
            {span.serviceName}
          </strong>
        </div>

        {/* Critical Path Badge */}
        {isCritical && (
          <span
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '2px',
              fontSize: '0.62rem',
              color: '#f59e0b',
              backgroundColor: 'rgba(245, 158, 11, 0.12)',
              padding: '2px 5px',
              borderRadius: '4px',
              fontWeight: 700,
            }}
          >
            <Zap size={10} />
            <span>CRITICAL</span>
          </span>
        )}
      </div>

      {/* Operation Name */}
      <div
        style={{
          color: 'var(--text-muted)',
          fontSize: '0.72rem',
          fontFamily: 'var(--font-mono)',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          marginBottom: '8px',
        }}
      >
        {span.name}
      </div>

      {/* Footer: Duration & HTTP Status */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderTop: '1px solid rgba(255, 255, 255, 0.05)',
          paddingTop: '6px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px', fontWeight: 600, color: 'var(--accent-emerald)' }}>
          <span>{durationMs}ms</span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
          {span.httpStatusCode && span.httpStatusCode > 0 && (
            <span
              className={`badge ${span.httpStatusCode >= 400 || isError ? 'critical' : 'healthy'}`}
              style={{ fontSize: '0.64rem', padding: '1px 5px' }}
            >
              HTTP {span.httpStatusCode}
            </span>
          )}
          {isError ? (
            <AlertCircle size={13} style={{ color: '#ef4444' }} />
          ) : (
            <CheckCircle2 size={13} style={{ color: '#10b981' }} />
          )}
        </div>
      </div>

      {/* Output Handle (Outgoing downstream calls) */}
      <Handle
        type="source"
        position={Position.Bottom}
        style={{ background: isCritical ? '#f59e0b' : 'var(--text-muted)', width: 8, height: 8 }}
      />
    </div>
  );
};

const nodeTypes = {
  spanNode: CustomSpanNode,
};

export const TraceDAGView: React.FC<TraceDAGViewProps> = ({
  spans,
  selectedSpanId,
  onSelectSpan,
  getServiceColor,
}) => {
  // 1. Compute Critical Path (longest duration chain through the DAG)
  const criticalPathSpanIds = useMemo(() => {
    if (spans.length === 0) return new Set<string>();

    const spanMap = new Map<string, Span>();
    const childrenMap = new Map<string, Span[]>();

    spans.forEach((s) => {
      spanMap.set(s.spanId, s);
      childrenMap.set(s.spanId, []);
    });

    const roots: Span[] = [];
    spans.forEach((s) => {
      if (s.parentSpanId && childrenMap.has(s.parentSpanId)) {
        childrenMap.get(s.parentSpanId)!.push(s);
      } else {
        roots.push(s);
      }
    });

    const findLongest = (curr: Span): { path: string[]; duration: number } => {
      const children = childrenMap.get(curr.spanId) || [];
      const currentDuration = Math.round(curr.durationNs / 1000000);
      if (children.length === 0) {
        return { path: [curr.spanId], duration: currentDuration };
      }

      let bestPath: string[] = [];
      let maxChildDur = -1;

      children.forEach((c) => {
        const res = findLongest(c);
        if (res.duration > maxChildDur) {
          maxChildDur = res.duration;
          bestPath = res.path;
        }
      });

      return {
        path: [curr.spanId, ...bestPath],
        duration: currentDuration + maxChildDur,
      };
    };

    let overallBest: string[] = [];
    let maxTotal = -1;
    roots.forEach((r) => {
      const res = findLongest(r);
      if (res.duration > maxTotal) {
        maxTotal = res.duration;
        overallBest = res.path;
      }
    });

    return new Set(overallBest);
  }, [spans]);

  // 2. Generate DAG Layout (levels and x, y coordinates)
  const { nodes, edges } = useMemo(() => {
    const spanMap = new Map<string, Span>();
    const childrenMap = new Map<string, Span[]>();

    spans.forEach((s) => {
      spanMap.set(s.spanId, s);
      childrenMap.set(s.spanId, []);
    });

    const roots: Span[] = [];
    spans.forEach((s) => {
      if (s.parentSpanId && childrenMap.has(s.parentSpanId)) {
        childrenMap.get(s.parentSpanId)!.push(s);
      } else {
        roots.push(s);
      }
    });

    // Level-by-level assignment
    const levels: Map<number, Span[]> = new Map();
    const spanLevelMap = new Map<string, number>();

    const assignLevels = (node: Span, level: number) => {
      spanLevelMap.set(node.spanId, level);
      const list = levels.get(level) || [];
      list.push(node);
      levels.set(level, list);

      const children = childrenMap.get(node.spanId) || [];
      children.forEach((c) => assignLevels(c, level + 1));
    };

    roots.forEach((r) => assignLevels(r, 0));

    const flowNodes: Node[] = [];
    const flowEdges: Edge[] = [];

    // Calculate layout coordinates
    const NODE_WIDTH = 250;
    const NODE_HEIGHT = 120;
    const HORIZONTAL_GAP = 60;
    const VERTICAL_GAP = 90;

    levels.forEach((spansInLevel, levelIndex) => {
      const totalLevelWidth = spansInLevel.length * (NODE_WIDTH + HORIZONTAL_GAP) - HORIZONTAL_GAP;
      const startX = -totalLevelWidth / 2;

      spansInLevel.forEach((s, itemIndex) => {
        const x = startX + itemIndex * (NODE_WIDTH + HORIZONTAL_GAP);
        const y = levelIndex * (NODE_HEIGHT + VERTICAL_GAP);
        const isCritical = criticalPathSpanIds.has(s.spanId);
        const isSelected = selectedSpanId === s.spanId;
        const durationMs = Math.round(s.durationNs / 1000000);
        const serviceColor = getServiceColor(s.serviceName, s.statusCode === 'ERROR');

        flowNodes.push({
          id: s.spanId,
          type: 'spanNode',
          position: { x, y },
          data: {
            span: s,
            durationMs,
            isCritical,
            isSelected,
            serviceColor,
          },
        });
      });
    });

    // Create DAG Edges
    spans.forEach((s) => {
      if (s.parentSpanId && spanMap.has(s.parentSpanId)) {
        const isEdgeCritical =
          criticalPathSpanIds.has(s.parentSpanId) && criticalPathSpanIds.has(s.spanId);

        flowEdges.push({
          id: `e-${s.parentSpanId}-${s.spanId}`,
          source: s.parentSpanId,
          target: s.spanId,
          animated: isEdgeCritical,
          style: {
            stroke: isEdgeCritical ? '#f59e0b' : 'rgba(255, 255, 255, 0.25)',
            strokeWidth: isEdgeCritical ? 2.5 : 1.5,
          },
          markerEnd: {
            type: MarkerType.ArrowClosed,
            color: isEdgeCritical ? '#f59e0b' : 'rgba(255, 255, 255, 0.4)',
            width: 16,
            height: 16,
          },
        });
      }
    });

    return { nodes: flowNodes, edges: flowEdges };
  }, [spans, criticalPathSpanIds, selectedSpanId, getServiceColor]);

  return (
    <div style={{ width: '100%', height: '540px', backgroundColor: '#0d1117' }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodeClick={(_, node) => {
          const nodeData = node.data as unknown as SpanNodeData;
          if (nodeData?.span) {
            onSelectSpan(nodeData.span);
          }
        }}
        fitView
        attributionPosition="bottom-right"
      >
        <Background color="rgba(255, 255, 255, 0.04)" gap={16} />
        <Controls
          style={{
            backgroundColor: 'var(--bg-card)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '8px',
          }}
        />
        <MiniMap
          nodeColor={(n) => {
            const data = n.data as unknown as SpanNodeData;
            return data?.serviceColor || '#3b82f6';
          }}
          maskColor="rgba(0, 0, 0, 0.6)"
          style={{
            backgroundColor: 'var(--bg-card)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '8px',
          }}
        />
      </ReactFlow>
    </div>
  );
};
