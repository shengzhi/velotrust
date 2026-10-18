import React from 'react';
import { Card } from './ui/card';
import { cn } from '../lib/utils';

export interface StatCardProps {
  title: string;
  value: number | string;
  icon: React.ReactNode;
  color?: 'blue' | 'green' | 'gray' | 'purple' | 'orange' | 'cyan';
  subtitle?: string;
  onClick?: () => void;
  className?: string;
}

export const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  icon,
  color = 'blue',
  subtitle,
  onClick,
  className,
}) => {
  const colorStyles = {
    blue: {
      bg: 'bg-blue-50/80',
      text: 'text-blue-600',
      border: 'border-blue-100',
      ring: 'group-hover:border-blue-200',
    },
    green: {
      bg: 'bg-emerald-50/80',
      text: 'text-emerald-600',
      border: 'border-emerald-100',
      ring: 'group-hover:border-emerald-200',
    },
    gray: {
      bg: 'bg-slate-100/80',
      text: 'text-slate-600',
      border: 'border-slate-200',
      ring: 'group-hover:border-slate-300',
    },
    purple: {
      bg: 'bg-purple-50/80',
      text: 'text-purple-600',
      border: 'border-purple-100',
      ring: 'group-hover:border-purple-200',
    },
    orange: {
      bg: 'bg-amber-50/80',
      text: 'text-amber-600',
      border: 'border-amber-100',
      ring: 'group-hover:border-amber-200',
    },
    cyan: {
      bg: 'bg-cyan-50/80',
      text: 'text-cyan-600',
      border: 'border-cyan-100',
      ring: 'group-hover:border-cyan-200',
    },
  };

  const currentStyle = colorStyles[color];

  return (
    <Card
      onClick={onClick}
      className={cn(
        'group relative overflow-hidden p-5 transition-all duration-200 border-slate-200/90',
        onClick && 'cursor-pointer hover:shadow-md hover:-translate-y-0.5',
        currentStyle.ring,
        className
      )}
    >
      <div className="flex items-start justify-between">
        <div>
          <span className="text-xs font-medium text-slate-500 uppercase tracking-wider">
            {title}
          </span>
          <div className="mt-2 text-2xl font-bold tracking-tight text-slate-900">
            {value}
          </div>
          {subtitle && (
            <p className="mt-1 text-xs text-slate-400 font-normal">{subtitle}</p>
          )}
        </div>
        <div
          className={cn(
            'flex h-11 w-11 items-center justify-center rounded-xl border transition-transform duration-200 group-hover:scale-105',
            currentStyle.bg,
            currentStyle.text,
            currentStyle.border
          )}
        >
          {icon}
        </div>
      </div>
    </Card>
  );
};
