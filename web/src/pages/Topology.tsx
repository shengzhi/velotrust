import React, { useState, useEffect, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  RefreshCw,
  LayoutGrid,
  Filter,
  Network,
  Share2,
  ChevronDown,
  ChevronUp,
  Server,
  Zap,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { Node, Site } from '../types';
import { TopologyGraph } from '../components/TopologyGraph';
import { GatewayBehindNetworkDiagram } from '../components/GatewayBehindNetworkDiagram';
import { Button } from '../components/ui/button';
import { BehindSubnetInfo, getBehindSubnetsForNode } from '../utils/networkTopology';

export const Topology: React.FC = () => {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [loading, setLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  // Selected gateway node whose backend network diagram is displayed
  const [selectedGateway, setSelectedGateway] = useState<Node | null>(null);
  const [selectedSubnetId, setSelectedSubnetId] = useState<string | null>(null);
  const [expandedGatewayIds, setExpandedGatewayIds] = useState<string[]>([]);
  const [layoutKey, setLayoutKey] = useState(0);
  // Mode toggle: by default true (Backbone only: Hub, Gateway, Relay)
  const [backboneOnly, setBackboneOnly] = useState(true);

  const navigate = useNavigate();

  const loadData = async (showRefresh = false) => {
    if (showRefresh) setIsRefreshing(true);
    try {
      const [nodesData, sitesData] = await Promise.all([
        mockApi.getNodes(),
        mockApi.getSites(),
      ]);
      setNodes(nodesData);
      setSites(sitesData);

      // Default select the first gateway or keep current
      const backbone = nodesData.filter((n) => n.type !== 'client');
      if (selectedGateway) {
        const updated = nodesData.find((n) => n.id === selectedGateway.id);
        if (updated) setSelectedGateway(updated);
      } else if (backbone.length > 0) {
        // Find first gateway or hub
        const defaultGw = backbone.find((n) => n.id === 'node-gw-sh') || backbone[0];
        setSelectedGateway(defaultGw);
        setExpandedGatewayIds([defaultGw.id]);
      }
    } finally {
      setLoading(false);
      setIsRefreshing(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  // Filter nodes: strictly exclude client agents in backbone mode
  const displayedNodes = useMemo(() => {
    if (!backboneOnly) return nodes;
    return nodes.filter((node) => {
      if (node.type === 'client') return false;
      const lowerId = node.id.toLowerCase();
      const lowerName = node.name.toLowerCase();
      if (
        lowerId.includes('client') ||
        lowerName.includes('client') ||
        lowerName.includes('macbook') ||
        lowerName.includes('thinkpad') ||
        lowerName.includes('ipad')
      ) {
        return false;
      }
      return true;
    });
  }, [nodes, backboneOnly]);

  // All gateway nodes available for selection
  const gatewayNodes = useMemo(() => {
    return displayedNodes.filter((n) => n.type === 'gateway');
  }, [displayedNodes]);

  // Count of client agents hidden
  const hiddenClientCount = useMemo(() => {
    return nodes.filter((n) => n.type === 'client').length;
  }, [nodes]);

  const handleRelayout = () => {
    setLayoutKey((prev) => prev + 1);
  };

  const handleToggleExpandGateway = (nodeId: string) => {
    setExpandedGatewayIds((prev) => {
      if (prev.includes(nodeId)) {
        return prev.filter((id) => id !== nodeId);
      } else {
        return [...prev, nodeId];
      }
    });
  };

  const handleExpandAll = () => {
    setExpandedGatewayIds(displayedNodes.map((n) => n.id));
  };

  const handleCollapseAll = () => {
    setExpandedGatewayIds([]);
  };

  // When clicking a node in the graph: update selected gateway for diagram view
  const handleNodeClick = (node: Node) => {
    setSelectedGateway(node);
    setSelectedSubnetId(null);
    if (!expandedGatewayIds.includes(node.id)) {
      setExpandedGatewayIds((prev) => [...prev, node.id]);
    }
  };

  const handleSubnetClick = (subnet: BehindSubnetInfo, parentNode: Node) => {
    setSelectedGateway(parentNode);
    setSelectedSubnetId(subnet.id);
    if (!expandedGatewayIds.includes(parentNode.id)) {
      setExpandedGatewayIds((prev) => [...prev, parentNode.id]);
    }
  };

  return (
    <div className="space-y-5">
      {/* Top Header & Controls Toolbar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-xl font-bold text-slate-900 tracking-tight">全局网络拓扑</h1>
            <span className="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-semibold bg-blue-50 text-blue-700 border border-blue-200">
              骨干拓扑视图
            </span>
          </div>
          <p className="text-xs text-slate-500 mt-1">
            仅展示 Hub、接入网关 (Gateway) 与中继 (Relay)；点击网关可在图谱中直接展开并查看其专属后端网络架构图（无遮挡弹框）
          </p>
        </div>

        {/* Action buttons & Viewport toggles */}
        <div className="flex flex-wrap items-center gap-2.5">
          {/* Backbone / All Nodes Filter Toggle */}
          <button
            onClick={() => setBackboneOnly(!backboneOnly)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors shadow-2xs ${
              backboneOnly
                ? 'bg-blue-50 border-blue-200 text-blue-700'
                : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-50'
            }`}
            title={backboneOnly ? '当前仅显示骨干基础设施节点' : '当前显示全量节点'}
          >
            <Filter className="w-3.5 h-3.5 text-blue-600" />
            <span>
              {backboneOnly
                ? `仅骨干节点 (${displayedNodes.length})`
                : `全网节点 (${nodes.length})`}
            </span>
            {backboneOnly && hiddenClientCount > 0 && (
              <span className="text-[10px] px-1.5 py-0.2 rounded-full bg-blue-100 text-blue-800">
                已排除 {hiddenClientCount} 个终端 Agent
              </span>
            )}
          </button>

          <Button
            variant="outline"
            size="sm"
            onClick={handleRelayout}
            className="gap-1.5 text-xs"
          >
            <LayoutGrid className="w-3.5 h-3.5" />
            <span>重新布局</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => loadData(true)}
            isLoading={isRefreshing}
            className="gap-1.5 text-xs"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin' : ''}`} />
            <span>刷新拓扑</span>
          </Button>
        </div>
      </div>

      {/* Main Canvas: Global WireGuard Mesh Topology */}
      <div className="relative">
        {loading ? (
          <div className="h-[600px] flex items-center justify-center bg-white rounded-xl border border-slate-200 shadow-2xs">
            <div className="flex flex-col items-center gap-3">
              <RefreshCw className="w-7 h-7 text-blue-600 animate-spin" />
              <p className="text-xs text-slate-500">正在生成骨干网关与后端网络拓扑...</p>
            </div>
          </div>
        ) : (
          <TopologyGraph
            key={layoutKey}
            nodes={displayedNodes}
            sites={sites}
            onNodeClick={handleNodeClick}
            onSubnetClick={handleSubnetClick}
            selectedNodeId={selectedGateway?.id}
            selectedSubnetId={selectedSubnetId}
            expandedGatewayIds={expandedGatewayIds}
            onToggleExpandGateway={handleToggleExpandGateway}
            onExpandAll={handleExpandAll}
            onCollapseAll={handleCollapseAll}
          />
        )}
      </div>

      {/* Gateway Backend Network Architecture Diagram (Visual diagram instead of popup modal) */}
      {selectedGateway && (
        <div className="animate-in fade-in duration-300">
          <GatewayBehindNetworkDiagram
            node={selectedGateway}
            sites={sites}
            allGatewayNodes={gatewayNodes}
            onSelectGateway={(gw) => handleNodeClick(gw)}
            onClose={() => setSelectedGateway(null)}
            onNavigateDetail={(id) => navigate(`/nodes/${id}`)}
          />
        </div>
      )}

      {/* Quick Helper Banner if no gateway is selected */}
      {!selectedGateway && !loading && (
        <div className="p-4 bg-slate-50 border border-slate-200 rounded-xl flex items-center justify-between text-xs text-slate-600">
          <div className="flex items-center gap-2">
            <Network className="w-4 h-4 text-blue-600" />
            <span>
              💡 <strong>提示</strong>：点击上方拓扑画布中的任意网关节点（如上海、深圳、北京等），即可在此处以图形化方式查看其专属后端局域网络架构图。
            </span>
          </div>
          {gatewayNodes.length > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleNodeClick(gatewayNodes[0])}
              className="text-xs h-7"
            >
              查看 {gatewayNodes[0].name} 后端网络
            </Button>
          )}
        </div>
      )}
    </div>
  );
};
