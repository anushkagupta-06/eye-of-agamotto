import React, { useEffect, useRef, useState, useMemo } from 'react';
import * as d3 from 'd3';
import { ZoomIn, ZoomOut, RotateCcw, Clock, ChevronRight, ChevronDown } from 'lucide-react';
import type { Span } from '../../types/telemetry';

export interface TraceFlamegraphProps {
  spans: Span[];
  selectedSpanId?: string;
  onSelectSpan: (span: Span) => void;
  serviceColors: Record<string, string>;
  getServiceColor: (service: string, isError: boolean) => string;
}

export interface FlamegraphNode {
  span: Span;
  depth: number;
  parent: Span | null;
  children: FlamegraphNode[];
  isSkewed: boolean;
  skewOffsetMs: number;
  durationMs: number;
  offsetMs: number;
}

export const TraceFlamegraphD3: React.FC<TraceFlamegraphProps> = ({
  spans,
  selectedSpanId,
  onSelectSpan,
  getServiceColor,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const [zoomLevel, setZoomLevel] = useState<number>(1);
  const [collapsedNodes, setCollapsedNodes] = useState<Set<string>>(new Set());

  // 1. Calculate overall trace bounds
  const traceStats = useMemo(() => {
    if (spans.length === 0) return { minTime: 0, maxDurationNs: 1, totalDurationMs: 0 };
    const minTime = Math.min(...spans.map((s) => new Date(s.startTime).getTime()));
    const maxEnd = Math.max(
      ...spans.map((s) => new Date(s.startTime).getTime() + Math.round(s.durationNs / 1000000))
    );
    const totalDurationMs = Math.max(maxEnd - minTime, 1);
    return { minTime, totalDurationMs };
  }, [spans]);

  // 2. Build hierarchical tree with Clock-Skew detection & clamping (Abstract Section E)
  const rootNodes = useMemo(() => {
    const nodeMap = new Map<string, FlamegraphNode>();

    spans.forEach((s) => {
      const startTime = new Date(s.startTime).getTime();
      const offsetMs = Math.max(0, startTime - traceStats.minTime);
      const durationMs = Math.max(Math.round(s.durationNs / 1000000), 1);

      nodeMap.set(s.spanId, {
        span: s,
        depth: 0,
        parent: null,
        children: [],
        isSkewed: false,
        skewOffsetMs: 0,
        durationMs,
        offsetMs,
      });
    });

    const roots: FlamegraphNode[] = [];

    spans.forEach((s) => {
      const node = nodeMap.get(s.spanId)!;
      if (s.parentSpanId && nodeMap.has(s.parentSpanId)) {
        const parentNode = nodeMap.get(s.parentSpanId)!;
        node.parent = parentNode.span;

        // Clock Skew Detection: Check if child starts before parent
        const parentStart = new Date(parentNode.span.startTime).getTime();
        const childStart = new Date(s.startTime).getTime();
        if (childStart < parentStart) {
          node.isSkewed = true;
          node.skewOffsetMs = parentStart - childStart;
          // Clamp child offset to parent window
          node.offsetMs = parentNode.offsetMs;
        }

        parentNode.children.push(node);
      } else {
        roots.push(node);
      }
    });

    // Sort chronologically and assign depths
    const assignDepths = (node: FlamegraphNode, depth: number) => {
      node.depth = depth;
      node.children.sort((a, b) => a.offsetMs - b.offsetMs);
      node.children.forEach((c) => assignDepths(c, depth + 1));
    };

    roots.sort((a, b) => a.offsetMs - b.offsetMs);
    roots.forEach((r) => assignDepths(r, 0));

    return roots;
  }, [spans, traceStats]);

  // 3. Flatten visible nodes taking collapsed state into account
  const flattenedNodes = useMemo(() => {
    const list: FlamegraphNode[] = [];
    const traverse = (node: FlamegraphNode) => {
      list.push(node);
      if (!collapsedNodes.has(node.span.spanId)) {
        node.children.forEach(traverse);
      }
    };
    rootNodes.forEach(traverse);
    return list;
  }, [rootNodes, collapsedNodes]);

  const toggleCollapse = (spanId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    setCollapsedNodes((prev) => {
      const next = new Set(prev);
      if (next.has(spanId)) {
        next.delete(spanId);
      } else {
        next.add(spanId);
      }
      return next;
    });
  };

  // 4. Render D3 Dynamic Time Axis
  useEffect(() => {
    if (!svgRef.current || traceStats.totalDurationMs <= 0) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = svgRef.current.clientWidth || 800;
    const height = 34;

    const xScale = d3
      .scaleLinear()
      .domain([0, traceStats.totalDurationMs / zoomLevel])
      .range([0, width]);

    const axis = d3
      .axisTop(xScale)
      .ticks(Math.max(width / 110, 4))
      .tickFormat((d) => `${Math.round(d as number)}ms`)
      .tickSize(6);

    const g = svg
      .attr('width', width)
      .attr('height', height)
      .append('g')
      .attr('transform', 'translate(0, 24)')
      .call(axis);

    // Style the D3 Axis to match dark theme
    g.select('.domain').attr('stroke', 'rgba(255, 255, 255, 0.15)');
    g.selectAll('.tick line').attr('stroke', 'rgba(255, 255, 255, 0.15)');
    g.selectAll('.tick text')
      .attr('fill', 'var(--text-muted)')
      .attr('font-size', '10px')
      .attr('font-family', 'var(--font-mono)');
  }, [traceStats.totalDurationMs, zoomLevel]);

  // Zoom handlers
  const handleZoomIn = () => setZoomLevel((z) => Math.min(z * 1.5, 10));
  const handleZoomOut = () => setZoomLevel((z) => Math.max(z / 1.5, 1));
  const handleResetZoom = () => setZoomLevel(1);

  return (
    <div ref={containerRef} style={{ display: 'flex', flexDirection: 'column', width: '100%' }}>
      {/* Flamegraph Controls Bar */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          padding: '8px 16px',
          backgroundColor: 'rgba(255, 255, 255, 0.02)',
          borderBottom: '1px solid var(--border-subtle)',
          fontSize: '0.74rem',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <span>Timeline Zoom: <strong>{zoomLevel.toFixed(1)}x</strong></span>
          {collapsedNodes.size > 0 && (
            <span className="code-pill" style={{ fontSize: '0.68rem', color: 'var(--accent-amber)' }}>
              {collapsedNodes.size} collapsed branches
            </span>
          )}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <button
            type="button"
            className="btn-secondary"
            onClick={handleZoomIn}
            title="Zoom into timeline"
            style={{ padding: '4px 8px', fontSize: '0.72rem', display: 'flex', alignItems: 'center', gap: '4px' }}
          >
            <ZoomIn size={13} />
            <span>Zoom In</span>
          </button>
          <button
            type="button"
            className="btn-secondary"
            onClick={handleZoomOut}
            title="Zoom out"
            style={{ padding: '4px 8px', fontSize: '0.72rem', display: 'flex', alignItems: 'center', gap: '4px' }}
          >
            <ZoomOut size={13} />
            <span>Zoom Out</span>
          </button>
          <button
            type="button"
            className="btn-secondary"
            onClick={handleResetZoom}
            title="Reset to 100% trace view"
            style={{ padding: '4px 8px', fontSize: '0.72rem', display: 'flex', alignItems: 'center', gap: '4px' }}
          >
            <RotateCcw size={13} />
            <span>Fit View</span>
          </button>
        </div>
      </div>

      {/* D3 Axis Ruler Header */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '380px 1fr',
          borderBottom: '1px solid var(--border-subtle)',
          backgroundColor: 'rgba(0, 0, 0, 0.25)',
        }}
      >
        <div
          style={{
            padding: '8px 16px',
            fontSize: '0.72rem',
            color: 'var(--text-muted)',
            fontWeight: 600,
            display: 'flex',
            alignItems: 'center',
            borderRight: '1px solid var(--border-subtle)',
          }}
        >
          Spans & Hierarchy
        </div>
        <div style={{ padding: '0 12px', overflow: 'hidden' }}>
          <svg ref={svgRef} style={{ width: '100%', height: '34px', display: 'block' }} />
        </div>
      </div>

      {/* Flamegraph Waterfall Rows */}
      <div style={{ maxHeight: '520px', overflowY: 'auto', backgroundColor: 'var(--bg-card)' }}>
        {flattenedNodes.map((node) => {
          const span = node.span;
          const isSelected = selectedSpanId === span.spanId;
          const isError = span.statusCode === 'ERROR';
          const hasChildren = node.children.length > 0;
          const isCollapsed = collapsedNodes.has(span.spanId);

          // Calculate bar position & width scaled with zoom
          const totalMs = Math.max(traceStats.totalDurationMs / zoomLevel, 1);
          const leftPercent = Math.min((node.offsetMs / totalMs) * 100, 96);
          const widthPercent = Math.max((node.durationMs / totalMs) * 100, 1.5);
          const serviceColor = getServiceColor(span.serviceName, isError);

          return (
            <div
              key={span.spanId}
              onClick={() => onSelectSpan(span)}
              style={{
                display: 'grid',
                gridTemplateColumns: '380px 1fr',
                borderBottom: '1px solid rgba(255, 255, 255, 0.04)',
                backgroundColor: isSelected
                  ? 'rgba(16, 185, 129, 0.12)'
                  : isError
                  ? 'rgba(239, 68, 68, 0.05)'
                  : 'transparent',
                cursor: 'pointer',
                transition: 'background-color 0.1s ease',
              }}
              onMouseEnter={(e) => {
                if (!isSelected) e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, 0.04)';
              }}
              onMouseLeave={(e) => {
                if (!isSelected) {
                  e.currentTarget.style.backgroundColor = isError ? 'rgba(239, 68, 68, 0.05)' : 'transparent';
                }
              }}
            >
              {/* Left Column: Tree Indentation & Details */}
              <div
                style={{
                  padding: '7px 12px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  borderRight: '1px solid var(--border-subtle)',
                  paddingLeft: `${14 + node.depth * 18}px`,
                  overflow: 'hidden',
                }}
              >
                {/* Collapse / Expand toggle */}
                {hasChildren ? (
                  <button
                    type="button"
                    onClick={(e) => toggleCollapse(span.spanId, e)}
                    style={{
                      background: 'none',
                      border: 'none',
                      padding: 0,
                      cursor: 'pointer',
                      color: 'var(--text-muted)',
                      display: 'flex',
                      alignItems: 'center',
                    }}
                  >
                    {isCollapsed ? <ChevronRight size={13} /> : <ChevronDown size={13} />}
                  </button>
                ) : (
                  <span style={{ width: '13px' }} />
                )}

                {/* Service Indicator Dot */}
                <span
                  style={{
                    width: '8px',
                    height: '8px',
                    borderRadius: '50%',
                    backgroundColor: serviceColor,
                    flexShrink: 0,
                  }}
                />

                {/* Service Name & Operation */}
                <div style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                    <span
                      style={{
                        fontWeight: 600,
                        fontSize: '0.78rem',
                        color: isError ? '#f87171' : 'var(--text-primary)',
                        whiteSpace: 'nowrap',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                      }}
                    >
                      {span.serviceName}
                    </span>

                    {/* Clock Skew Badge */}
                    {node.isSkewed && (
                      <span
                        className="badge warning"
                        title={`Clock skew detected: child reported starting ${node.skewOffsetMs}ms before parent. Clamped to parent window.`}
                        style={{ fontSize: '0.62rem', padding: '1px 4px', display: 'flex', alignItems: 'center', gap: '2px' }}
                      >
                        <Clock size={10} />
                        <span>skewed</span>
                      </span>
                    )}
                  </div>

                  <span
                    style={{
                      fontSize: '0.7rem',
                      color: 'var(--text-muted)',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    {span.name}
                  </span>
                </div>

                {/* HTTP Status Code Pill */}
                {span.httpStatusCode && span.httpStatusCode > 0 && (
                  <span
                    className={`badge ${span.httpStatusCode >= 400 || isError ? 'critical' : 'healthy'}`}
                    style={{
                      marginLeft: 'auto',
                      fontSize: '0.66rem',
                      padding: '1px 5px',
                      fontFamily: 'var(--font-mono)',
                    }}
                  >
                    {span.httpStatusCode}
                  </span>
                )}
              </div>

              {/* Right Column: D3 Waterfall Span Bar */}
              <div
                style={{
                  position: 'relative',
                  padding: '5px 12px',
                  display: 'flex',
                  alignItems: 'center',
                  overflow: 'hidden',
                }}
              >
                {/* Horizontal Guide Line */}
                <div
                  style={{
                    position: 'absolute',
                    top: '50%',
                    left: 0,
                    right: 0,
                    height: '1px',
                    backgroundColor: 'rgba(255, 255, 255, 0.03)',
                  }}
                />

                {/* Span Flame Bar */}
                <div
                  style={{
                    position: 'relative',
                    marginLeft: `${leftPercent}%`,
                    width: `${widthPercent}%`,
                    minWidth: '12px',
                    height: '20px',
                    borderRadius: '4px',
                    backgroundColor: serviceColor,
                    boxShadow: isError
                      ? '0 0 10px rgba(239, 68, 68, 0.7)'
                      : isSelected
                      ? '0 0 8px rgba(16, 185, 129, 0.6)'
                      : 'none',
                    border: isSelected
                      ? '1.5px solid #fff'
                      : isError
                      ? '1px solid #f87171'
                      : '1px solid rgba(255, 255, 255, 0.15)',
                    display: 'flex',
                    alignItems: 'center',
                    padding: '0 6px',
                    overflow: 'hidden',
                    transition: 'all 0.15s ease',
                  }}
                  title={`${span.serviceName} - ${span.name}\nDuration: ${node.durationMs}ms\nOffset: ${node.offsetMs}ms`}
                >
                  <span
                    style={{
                      fontSize: '0.68rem',
                      fontWeight: 600,
                      color: '#fff',
                      whiteSpace: 'nowrap',
                      textShadow: '0 1px 2px rgba(0, 0, 0, 0.8)',
                    }}
                  >
                    {node.durationMs}ms
                  </span>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
