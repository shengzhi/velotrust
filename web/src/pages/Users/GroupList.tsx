import React from 'react';
import { UsersRound } from 'lucide-react';
import { Card } from '../../components/ui/card';
import { EmptyState } from '../../components/EmptyState';

export const GroupList: React.FC = () => (
  <div className="space-y-5">
    <div><h1 className="text-xl font-bold tracking-tight text-slate-900">用户组管理</h1><p className="mt-1 text-xs text-slate-500">用户组用于后续按业务范围组织账号与设备。</p></div>
    <Card className="p-8">
      <EmptyState icon={UsersRound} title="用户组 JRPC 尚未实现" description="当前 Management IAM 仅提供用户与角色管理接口。此页面不会使用原型数据创建或修改用户组。" />
    </Card>
  </div>
);
