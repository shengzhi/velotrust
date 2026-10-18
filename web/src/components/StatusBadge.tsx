import React from 'react';
import { cn } from '../lib/utils';

export type GeneralStatus =
  | 'online'
  | 'active'
  | 'allow'
  | 'offline'
  | 'revoked'
  | 'deny'
  | 'degraded'
  | 'warning'
  | 'disabled'
  | 'inactive';

interface StatusBadgeProps {
  status: string;
  label?: string;
  className?: string;
  showDot?: boolean;
}

export const StatusBadge: React.FC<StatusBadgeProps> = ({
  status,
  label,
  className,
  showDot = true,
}) => {
  const norm = status.toLowerCase() as GeneralStatus;

  let colorClasses = 'bg-slate-100 text-slate-600 border-slate-200';
  let dotColor = 'bg-slate-400';
  let defaultLabel = status;

  if (['online', 'active', 'allow', '启用'].includes(norm)) {
    colorClasses = 'bg-emerald-50 text-emerald-700 border-emerald-200/80';
    dotColor = 'bg-emerald-500';
    defaultLabel = norm === 'online' ? '在线' : norm === 'allow' ? '允许' : '正常';
  } else if (['offline', 'revoked', 'deny', '禁用'].includes(norm)) {
    colorClasses = 'bg-rose-50 text-rose-700 border-rose-200/80';
    dotColor = 'bg-rose-500';
    defaultLabel = norm === 'offline' ? '离线' : norm === 'revoked' ? '已撤销' : norm === 'deny' ? '拒绝' : '禁用';
  } else if (['degraded', 'warning'].includes(norm)) {
    colorClasses = 'bg-amber-50 text-amber-700 border-amber-200/80';
    dotColor = 'bg-amber-500';
    defaultLabel = norm === 'degraded' ? '降级' : '警告';
  } else if (['pending_enrollment', 'pending', '待入网', '待注册'].includes(norm)) {
    colorClasses = 'bg-blue-50 text-blue-700 border-blue-200/80';
    dotColor = 'bg-blue-500';
    defaultLabel = '待注册入网';
  } else if (['disabled', 'inactive'].includes(norm)) {
    colorClasses = 'bg-slate-100 text-slate-500 border-slate-200';
    dotColor = 'bg-slate-400';
    defaultLabel = norm === 'disabled' ? '已禁用' : '未激活';
  }

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium border shadow-2xs select-none',
        colorClasses,
        className
      )}
    >
      {showDot && (
        <span className="relative flex h-2 w-2">
          {['online', 'active', 'allow'].includes(norm) && (
            <span
              className={cn(
                'animate-ping absolute inline-flex h-full w-full rounded-full opacity-60',
                dotColor
              )}
            />
          )}
          <span className={cn('relative inline-flex rounded-full h-2 w-2', dotColor)} />
        </span>
      )}
      <span>{label || defaultLabel}</span>
    </span>
  );
};
