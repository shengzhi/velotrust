import React, { useState } from 'react';
import { Node, Site } from '../types';
import {
  Server,
  ShieldCheck,
  Zap,
  Network,
  Radio,
  Plus,
  Minus,
  Maximize2,
  ChevronDown,
  ChevronUp,
  FolderTree,
  Share2,
} from 'lucide-react';
import { BehindSubnetInfo, getBehindSubnetsForNode } from '../utils/networkTopology';

export interface TopologyGraphProps {
  nodes: Node[];
  sites?: Site[];
  onNodeClick: (node: Node) => void;
  onSubnetClick?: (subnet: BehindSubnetInfo, parentNode: Node) => void;
  selectedNodeId?: string | null;
  selectedSubnetId?: string | null;
  expandedGatewayIds?: string[];
  onToggleExpandGateway?: (nodeId: string) => void;
  onExpandAll?: () => void;
  onCollapseAll?: () => void;
}

export const TopologyGraph: React.FC<TopologyGraphProps> = ({
  nodes,
  sites = [],
  onNodeClick,
  onSubnetClick,
  selectedNodeId,
  selectedSubnetId,
  expandedGatewayIds = [],
  onToggleExpandGateway,
  onExpandAll,
  onCollapseAll,
}) => {
  const [hoveredNodeId, setHoveredNodeId] = useState<string | null>(null);
  const [hoveredSubnetId, setHoveredSubnetId] = useState<string | null>(null);
  const [zoomScale, setZoomScale] = useState(1);

  // SVG canvas dimensions
  const width = 1000;
  const height = 680;
  const hubX = 500;
  const hubY = 340;
  const orbitRadius = 215;

  // Identify central hub (usually node named hub or id with hub or first gateway)
  const hubNode =
    nodes.find(
      (n) =>
        n.id === 'node-hub-bj' ||
        n.name.toLowerCase().includes('hub') ||
        n.nodeTypeLabel?.includes('Hub')
    ) || nodes[0];

  // Satellite nodes are other gateways/relays
  const satelliteNodes = nodes.filter((n) => n.id !== (hubNode ? hubNode.id : ''));
  const totalSatellites = satelliteNodes.length;

  // Compute satellite positions evenly in circle
  const positionedSatellites = satelliteNodes.map((node, index) => {
    // Angle in radians (start from top and rotate clockwise)
    const angle = (index / (totalSatellites || 1)) * 2 * Math.PI - Math.PI / 2;
    const x = hubX + orbitRadius * Math.cos(angle);
    const y = hubY + orbitRadius * Math.sin(angle);
    const subnets = getBehindSubnetsForNode(node, sites);
    const isExpanded = expandedGatewayIds.includes(node.id);

    return {
      node,
      angle,
      x,
      y,
      subnets,
      isExpanded,
      isDirect: node.status === 'online',
    };
  });

  // Calculate hub's subnets if hub is expanded
  const hubSubnets = hubNode ? getBehindSubnetsForNode(hubNode, sites) : [];
  const isHubExpanded = hubNode ? expandedGatewayIds.includes(hubNode.id) : false;

  const getStatusColor = (status: Node['status']) => {
    switch (status) {
      case 'online':
        return {
          fill: '#ecfdf5',
          stroke: '#10b981',
          text: '#065f46',
          badge: '#10b981',
          halo: 'rgba(16, 185, 129, 0.22)',
        };
      case 'degraded':
        return {
          fill: '#fffbeb',
          stroke: '#f59e0b',
          text: '#92400e',
          badge: '#f59e0b',
          halo: 'rgba(245, 158, 11, 0.22)',
        };
      case 'offline':
      default:
        return {
          fill: '#f8fafc',
          stroke: '#94a3b8',
          text: '#475569',
          badge: '#94a3b8',
          halo: 'transparent',
        };
    }
  };

  const handleNodeClick = (node: Node) => {
    onNodeClick(node);
    if (onToggleExpandGateway) {
      onToggleExpandGateway(node.id);
    }
  };

  const handleZoom = (delta: number) => {
    setZoomScale((prev) => Math.min(1.4, Math.max(0.7, prev + delta)));
  };

  const handleResetZoom = () => {
    setZoomScale(1);
  };

  // Total behind subnets count across all nodes
  const totalSubnetsCount = nodes.reduce((acc, n) => {
    return acc + getBehindSubnetsForNode(n, sites).length;
  }, 0);

  const areAllExpanded =
    nodes.length > 0 && nodes.every((n) => expandedGatewayIds.includes(n.id));

  return (
    <div className="relative w-full h-[650px] overflow-hidden rounded-xl border border-slate-200 bg-white select-none shadow-2xs">
      {/* SVG Canvas */}
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="w-full h-full transition-transform duration-300"
        style={{
          transform: `scale(${zoomScale})`,
          transformOrigin: '50% 50%',
        }}
        preserveAspectRatio="xMidYMid meet"
      >
        <defs>
          {/* Subtle architectural dot grid */}
          <pattern id="meshGrid" width="36" height="36" patternUnits="userSpaceOnUse">
            <path d="M 36 0 L 0 0 0 36" fill="none" stroke="#f8fafc" strokeWidth="1" />
            <circle cx="18" cy="18" r="0.9" fill="#e2e8f0" />
          </pattern>

          {/* Glow filter */}
          <filter id="hubGlow" x="-20%" y="-20%" width="140%" height="140%">
            <feGaussianBlur stdDeviation="4" result="blur" />
            <feComposite in="SourceGraphic" in2="blur" operator="over" />
          </filter>

          {/* Card drop shadow */}
          <filter id="cardShadow" x="-10%" y="-10%" width="120%" height="130%">
            <feDropShadow dx="0" dy="2" stdDeviation="2.5" floodColor="#0f172a" floodOpacity="0.08" />
          </filter>
        </defs>

        {/* Background Grid */}
        <rect width={width} height={height} fill="url(#meshGrid)" />

        {/* Orbit track ring */}
        <circle
          cx={hubX}
          cy={hubY}
          r={orbitRadius}
          fill="none"
          stroke="#f1f5f9"
          strokeWidth="1.5"
          strokeDasharray="5 5"
        />

        {/* Outer safety guide ring */}
        <circle
          cx={hubX}
          cy={hubY}
          r={orbitRadius + 115}
          fill="none"
          stroke="#f8fafc"
          strokeWidth="1"
          strokeDasharray="3 3"
        />

        {/* --- 1. WireGuard Mesh Tunnel Links (Hub to Satellites) --- */}
        {hubNode &&
          positionedSatellites.map(({ node, x, y, isDirect }) => {
            const isHovered = hoveredNodeId === node.id || hoveredNodeId === hubNode.id;
            const isSelected = selectedNodeId === node.id;
            const isOffline = node.status === 'offline';

            return (
              <g key={`edge-${node.id}`}>
                <line
                  x1={hubX}
                  y1={hubY}
                  x2={x}
                  y2={y}
                  stroke={
                    isSelected
                      ? '#2563eb'
                      : isHovered
                        ? '#3b82f6'
                        : isOffline
                          ? '#cbd5e1'
                          : isDirect
                            ? '#60a5fa'
                            : '#94a3b8'
                  }
                  strokeWidth={isSelected ? 3 : isHovered ? 2.5 : isDirect ? 2 : 1.5}
                  strokeDasharray={isDirect ? 'none' : '4 4'}
                  strokeOpacity={isOffline ? 0.35 : isHovered || isSelected ? 1 : 0.75}
                  className="transition-all duration-300"
                />

                {/* Animated data packet indicator on active links */}
                {!isOffline && (
                  <circle r="3" fill="#3b82f6" opacity="0.85">
                    <animateMotion
                      path={`M ${hubX} ${hubY} L ${x} ${y}`}
                      dur={isDirect ? '3.5s' : '5.5s'}
                      repeatCount="indefinite"
                    />
                  </circle>
                )}
              </g>
            );
          })}

        {/* --- 2. Behind Networks / Subnets of Satellites (When Expanded) --- */}
        {positionedSatellites.map(({ node, angle, x, y, subnets, isExpanded }) => {
          if (!isExpanded || subnets.length === 0) return null;

          const numSubnets = subnets.length;
          const dist = 115;

          return (
            <g key={`behind-nets-${node.id}`} className="transition-all duration-300">
              {subnets.map((subnet, idx) => {
                // Calculate fan-out angle outward from hub through gateway
                let subAngle = angle;
                if (numSubnets === 2) {
                  subAngle = angle + (idx === 0 ? -0.26 : 0.26);
                } else if (numSubnets === 3) {
                  subAngle = angle + (idx - 1) * 0.28;
                } else if (numSubnets > 3) {
                  subAngle = angle + (idx - (numSubnets - 1) / 2) * 0.24;
                }

                // Clamped coordinates for subnet pod
                let sx = x + dist * Math.cos(subAngle);
                let sy = y + dist * Math.sin(subAngle);

                // Margin clamp to keep within canvas
                sx = Math.max(90, Math.min(width - 90, sx));
                sy = Math.max(45, Math.min(height - 45, sy));

                const isSubnetHovered = hoveredSubnetId === subnet.id;
                const isSubnetSelected = selectedSubnetId === subnet.id;

                // Color themes based on subnet category
                const isNat = subnet.type === 'nat_mapped';
                const isLocal = subnet.type === 'local';

                const branchStroke = isNat ? '#8b5cf6' : isLocal ? '#10b981' : '#0284c7';
                const podBg = isSubnetSelected
                  ? isNat
                    ? '#f5f3ff'
                    : isLocal
                      ? '#ecfdf5'
                      : '#eff6ff'
                  : '#ffffff';
                const podBorder = isSubnetSelected
                  ? branchStroke
                  : isSubnetHovered
                    ? branchStroke
                    : isNat
                      ? '#ddd6fe'
                      : isLocal
                        ? '#a7f3d0'
                        : '#bae6fd';

                // Midpoint control for curved organic branch
                const mx = (x * 1.2 + sx) / 2.2;
                const my = (y * 1.2 + sy) / 2.2;

                const podW = 144;
                const podH = 38;

                return (
                  <g key={subnet.id} className="cursor-pointer">
                    {/* Branch link from Gateway to Behind Subnet */}
                    <path
                      d={`M ${x} ${y} Q ${mx} ${my} ${sx} ${sy}`}
                      fill="none"
                      stroke={branchStroke}
                      strokeWidth={isSubnetSelected ? 2.5 : 1.75}
                      strokeDasharray="4 3"
                      strokeOpacity={isSubnetHovered || isSubnetSelected ? 1 : 0.8}
                    />

                    {/* Animated packet towards subnet */}
                    <circle r="2.5" fill={branchStroke}>
                      <animateMotion
                        path={`M ${x} ${y} Q ${mx} ${my} ${sx} ${sy}`}
                        dur="3s"
                        repeatCount="indefinite"
                      />
                    </circle>

                    {/* Subnet Pod Card */}
                    <g
                      transform={`translate(${sx - podW / 2}, ${sy - podH / 2})`}
                      onClick={(e) => {
                        e.stopPropagation();
                        if (onSubnetClick) {
                          onSubnetClick(subnet, node);
                        }
                      }}
                      onMouseEnter={() => setHoveredSubnetId(subnet.id)}
                      onMouseLeave={() => setHoveredSubnetId(null)}
                    >
                      {/* Pod Background */}
                      <rect
                        width={podW}
                        height={podH}
                        rx="8"
                        ry="8"
                        fill={podBg}
                        stroke={podBorder}
                        strokeWidth={isSubnetSelected ? 2 : 1.2}
                        filter="url(#cardShadow)"
                        className="transition-all duration-200"
                      />

                      {/* Type icon dot */}
                      <circle
                        cx={14}
                        cy={podH / 2}
                        r={4}
                        fill={branchStroke}
                      />

                      {/* CIDR text */}
                      <text
                        x={24}
                        y={16}
                        className="text-[11px] font-mono font-bold fill-slate-800 pointer-events-none"
                      >
                        {subnet.cidr}
                      </text>

                      {/* Subnet label / role */}
                      <text
                        x={24}
                        y={29}
                        className="text-[9px] font-medium fill-slate-500 pointer-events-none"
                      >
                        {isNat
                          ? '1:1 NAT 影子网段'
                          : isLocal
                            ? '本地原生 VPC/私网'
                            : 'AllowedIPs 广播'}
                      </text>

                      {/* Subtle status dot */}
                      <circle
                        cx={podW - 12}
                        cy={12}
                        r={2.5}
                        fill="#10b981"
                      />
                    </g>
                  </g>
                );
              })}
            </g>
          );
        })}

        {/* --- 3. Behind Networks of Hub (When Hub is Expanded) --- */}
        {isHubExpanded && hubSubnets.length > 0 && (
          <g key="hub-behind-subnets">
            {hubSubnets.map((subnet, idx) => {
              // Position subnets around Hub (e.g. above and below)
              const offsetAngle = -Math.PI / 2 + (idx - (hubSubnets.length - 1) / 2) * 0.45;
              const dist = 95;
              const sx = hubX + dist * Math.cos(offsetAngle);
              const sy = hubY + dist * Math.sin(offsetAngle) - 25;

              const isSubnetHovered = hoveredSubnetId === subnet.id;
              const isSubnetSelected = selectedSubnetId === subnet.id;
              const branchStroke = '#3b82f6';
              const podW = 140;
              const podH = 36;

              return (
                <g
                  key={subnet.id}
                  className="cursor-pointer"
                  onClick={(e) => {
                    e.stopPropagation();
                    if (hubNode && onSubnetClick) {
                      onSubnetClick(subnet, hubNode);
                    }
                  }}
                  onMouseEnter={() => setHoveredSubnetId(subnet.id)}
                  onMouseLeave={() => setHoveredSubnetId(null)}
                >
                  <line
                    x1={hubX}
                    y1={hubY}
                    x2={sx}
                    y2={sy}
                    stroke={branchStroke}
                    strokeWidth="1.5"
                    strokeDasharray="3 3"
                    strokeOpacity={0.8}
                  />
                  <g transform={`translate(${sx - podW / 2}, ${sy - podH / 2})`}>
                    <rect
                      width={podW}
                      height={podH}
                      rx="7"
                      ry="7"
                      fill={isSubnetSelected ? '#eff6ff' : '#ffffff'}
                      stroke={isSubnetSelected ? '#2563eb' : '#93c5fd'}
                      strokeWidth={isSubnetSelected ? 2 : 1.2}
                      filter="url(#cardShadow)"
                    />
                    <circle cx={14} cy={podH / 2} r={3.5} fill="#2563eb" />
                    <text
                      x={24}
                      y={15}
                      className="text-[11px] font-mono font-bold fill-slate-800 pointer-events-none"
                    >
                      {subnet.cidr}
                    </text>
                    <text
                      x={24}
                      y={28}
                      className="text-[9px] font-medium fill-slate-500 pointer-events-none"
                    >
                      {subnet.name.slice(0, 14)}
                    </text>
                  </g>
                </g>
              );
            })}
          </g>
        )}

        {/* --- 4. Satellite Nodes (Gateways & Relays) --- */}
        {positionedSatellites.map(({ node, x, y, subnets, isExpanded }) => {
          const colors = getStatusColor(node.status);
          const isSelected = selectedNodeId === node.id;
          const isHovered = hoveredNodeId === node.id;
          const r = 26;

          // Determine node icon and title
          const isRelay =
            node.nodeTypeLabel?.includes('Relay') ||
            node.nodeTypeLabel?.includes('中继') ||
            node.name.includes('iot') ||
            node.name.includes('fra');

          const nodeBadgeText = isRelay
            ? '中继网关 (Relay)'
            : node.type === 'gateway'
              ? '接入网关'
              : 'Mesh 节点';

          return (
            <g
              key={node.id}
              transform={`translate(${x}, ${y})`}
              className="cursor-pointer transition-transform duration-200"
              onClick={() => handleNodeClick(node)}
              onMouseEnter={() => setHoveredNodeId(node.id)}
              onMouseLeave={() => setHoveredNodeId(null)}
            >
              {/* Halo circle */}
              {(isHovered || isSelected || node.status === 'online') && (
                <circle
                  r={r + 8}
                  fill={colors.halo}
                  className={node.status === 'online' ? 'animate-pulse' : ''}
                />
              )}

              {/* Main Node Circle */}
              <circle
                r={r}
                fill={colors.fill}
                stroke={isSelected ? '#2563eb' : colors.stroke}
                strokeWidth={isSelected ? 3.5 : isHovered ? 2.5 : 2}
                className="transition-all"
              />

              {/* Status Dot badge */}
              <circle
                cx={r - 4}
                cy={-r + 4}
                r={5}
                fill={colors.badge}
                stroke="#ffffff"
                strokeWidth={1.5}
              />

              {/* Node Icon inside circle */}
              <foreignObject
                x={-12}
                y={-12}
                width={24}
                height={24}
                className="pointer-events-none"
              >
                <div className="w-full h-full flex items-center justify-center">
                  {isRelay ? (
                    <Radio
                      className="w-4 h-4"
                      style={{ color: isSelected ? '#2563eb' : colors.stroke }}
                    />
                  ) : (
                    <Server
                      className="w-4 h-4"
                      style={{ color: isSelected ? '#2563eb' : colors.stroke }}
                    />
                  )}
                </div>
              </foreignObject>

              {/* Node Name Label */}
              <text
                y={r + 14}
                textAnchor="middle"
                className={`text-[11px] font-medium transition-colors ${
                  isSelected ? 'fill-blue-700 font-bold' : 'fill-slate-800'
                }`}
              >
                {node.name.length > 15 ? node.name.slice(0, 13) + '...' : node.name}
              </text>

              {/* Virtual IP & Node Type */}
              <text
                y={r + 26}
                textAnchor="middle"
                className="text-[9px] fill-slate-500 font-mono"
              >
                {node.virtualIp.split('/')[0]} · {nodeBadgeText}
              </text>

              {/* Interactive Backend Subnets Pill */}
              {subnets.length > 0 && (
                <g
                  transform={`translate(0, ${r + 34})`}
                  className="cursor-pointer group"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleNodeClick(node);
                  }}
                >
                  <rect
                    x={-46}
                    y={0}
                    width={92}
                    height={18}
                    rx="9"
                    ry="9"
                    fill={isExpanded ? '#1d4ed8' : isSelected ? '#2563eb' : '#f1f5f9'}
                    stroke={isExpanded || isSelected ? '#1e40af' : '#cbd5e1'}
                    strokeWidth="1"
                    className="transition-colors"
                  />
                  <text
                    x={0}
                    y={12}
                    textAnchor="middle"
                    className={`text-[9px] font-medium transition-colors ${
                      isExpanded || isSelected
                        ? 'fill-white font-semibold'
                        : 'fill-slate-600 group-hover:fill-blue-600'
                    }`}
                  >
                    {isExpanded ? `▲ 收起网段` : `▼ ${subnets.length} 个后端网段`}
                  </text>
                </g>
              )}
            </g>
          );
        })}

        {/* --- 5. Central Hub Node (Special Styling) --- */}
        {hubNode && (() => {
          const colors = getStatusColor(hubNode.status);
          const isSelected = selectedNodeId === hubNode.id;
          const isHovered = hoveredNodeId === hubNode.id;
          const r = 36;

          return (
            <g
              transform={`translate(${hubX}, ${hubY})`}
              className="cursor-pointer"
              onClick={() => handleNodeClick(hubNode)}
              onMouseEnter={() => setHoveredNodeId(hubNode.id)}
              onMouseLeave={() => setHoveredNodeId(null)}
            >
              {/* Outer pulsing ring for Hub */}
              <circle
                r={r + 14}
                fill="none"
                stroke="#3b82f6"
                strokeWidth="1.5"
                opacity="0.35"
                className="animate-ping"
              />
              <circle r={r + 8} fill="rgba(37, 99, 235, 0.12)" />

              {/* Main Hub circle */}
              <circle
                r={r}
                fill="#ffffff"
                stroke={isSelected ? '#1d4ed8' : '#2563eb'}
                strokeWidth={isSelected ? 4 : 3}
                filter="url(#hubGlow)"
              />

              {/* Status Badge */}
              <circle
                cx={r - 5}
                cy={-r + 5}
                r={6}
                fill={colors.badge}
                stroke="#ffffff"
                strokeWidth={2}
              />

              {/* Hub Icon */}
              <foreignObject
                x={-16}
                y={-16}
                width={32}
                height={32}
                className="pointer-events-none"
              >
                <div className="w-full h-full flex items-center justify-center">
                  <ShieldCheck className="w-6 h-6 text-blue-600" />
                </div>
              </foreignObject>

              {/* Hub Label */}
              <text
                y={r + 16}
                textAnchor="middle"
                className="text-xs font-bold fill-slate-900"
              >
                {hubNode.name}
              </text>
              <text
                y={r + 29}
                textAnchor="middle"
                className="text-[10px] fill-blue-600 font-mono font-medium"
              >
                {hubNode.virtualIp} · 核心中枢网关 (Hub)
              </text>

              {/* Hub backend subnets indicator pill */}
              {hubSubnets.length > 0 && (
                <g
                  transform={`translate(0, ${r + 36})`}
                  className="cursor-pointer"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleNodeClick(hubNode);
                  }}
                >
                  <rect
                    x={-44}
                    y={0}
                    width={88}
                    height={18}
                    rx="9"
                    ry="9"
                    fill={isHubExpanded ? '#1d4ed8' : '#eff6ff'}
                    stroke={isHubExpanded ? '#1e40af' : '#bfdbfe'}
                    strokeWidth="1"
                  />
                  <text
                    x={0}
                    y={12}
                    textAnchor="middle"
                    className={`text-[9px] font-medium ${
                      isHubExpanded ? 'fill-white font-semibold' : 'fill-blue-700'
                    }`}
                  >
                    {isHubExpanded ? '▲ 收起网段' : `▼ ${hubSubnets.length} 个后端网段`}
                  </text>
                </g>
              )}
            </g>
          );
        })()}
      </svg>

      {/* Floating Info & Quick Action Header (Top-Left) */}
      <div className="absolute top-4 left-4 flex flex-wrap items-center gap-2">
        <div className="flex items-center gap-2 bg-white/95 backdrop-blur-xs border border-slate-200 px-3 py-1.5 rounded-lg text-xs shadow-2xs">
          <Zap className="w-3.5 h-3.5 text-blue-600" />
          <span className="text-slate-600">
            骨干网关与中继节点: <strong className="text-slate-900">{nodes.length}</strong>
          </span>
          <span className="text-slate-300">|</span>
          <span className="text-slate-600">
            总代理后端网段: <strong className="text-blue-600 font-mono">{totalSubnetsCount}</strong>
          </span>
        </div>

        {/* Expand/Collapse All Button */}
        {onExpandAll && onCollapseAll && (
          <button
            onClick={areAllExpanded ? onCollapseAll : onExpandAll}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white/95 backdrop-blur-xs border border-slate-200 text-xs font-medium text-slate-700 hover:text-blue-600 hover:bg-slate-50 transition-colors shadow-2xs"
          >
            {areAllExpanded ? (
              <>
                <ChevronUp className="w-3.5 h-3.5 text-slate-500" />
                <span>收起所有后端网络</span>
              </>
            ) : (
              <>
                <FolderTree className="w-3.5 h-3.5 text-blue-600" />
                <span>展开所有网关后端网络</span>
              </>
            )}
          </button>
        )}
      </div>

      {/* Zoom & Viewport Controls (Bottom-Right) */}
      <div className="absolute bottom-4 right-4 flex items-center gap-1.5 bg-white/95 backdrop-blur-xs border border-slate-200 p-1 rounded-lg shadow-2xs text-xs">
        <button
          onClick={() => handleZoom(0.1)}
          className="p-1.5 text-slate-600 hover:text-slate-900 hover:bg-slate-100 rounded transition-colors"
          title="放大"
        >
          <Plus className="w-3.5 h-3.5" />
        </button>
        <span className="font-mono text-[11px] text-slate-500 px-1 min-w-[36px] text-center">
          {Math.round(zoomScale * 100)}%
        </span>
        <button
          onClick={() => handleZoom(-0.1)}
          className="p-1.5 text-slate-600 hover:text-slate-900 hover:bg-slate-100 rounded transition-colors"
          title="缩小"
        >
          <Minus className="w-3.5 h-3.5" />
        </button>
        <div className="w-px h-3.5 bg-slate-200 mx-0.5" />
        <button
          onClick={handleResetZoom}
          className="p-1.5 text-slate-600 hover:text-slate-900 hover:bg-slate-100 rounded transition-colors"
          title="重置视图"
        >
          <Maximize2 className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* Bottom-Left Legend */}
      <div className="hidden sm:flex absolute bottom-4 left-4 items-center gap-3 px-3 py-1.5 rounded-lg bg-white/95 backdrop-blur-xs border border-slate-200 text-[11px] text-slate-600 shadow-2xs">
        <span className="text-slate-400 font-medium">网段图例：</span>
        <div className="flex items-center gap-1">
          <span className="w-2.5 h-2.5 rounded-sm bg-emerald-500" />
          <span>本地原生 VPC</span>
        </div>
        <div className="flex items-center gap-1">
          <span className="w-2.5 h-2.5 rounded-sm bg-purple-500" />
          <span>1:1 NAT 影子网段</span>
        </div>
        <div className="flex items-center gap-1">
          <span className="w-2.5 h-2.5 rounded-sm bg-sky-500" />
          <span>通告网段</span>
        </div>
        <div className="h-3 w-px bg-slate-200 mx-0.5" />
        <span className="text-slate-400">💡 点击网关节点可展开其背后的具体网络段</span>
      </div>
    </div>
  );
};
