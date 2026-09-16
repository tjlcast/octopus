'use client';

import { useCallback, useMemo, useState } from 'react';
import { Ban, Check, Loader, Pencil, Plus, ShieldAlert, Trash2, X } from 'lucide-react';
import { PageWrapper } from '@/components/common/PageWrapper';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import {
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { toast } from '@/components/common/Toast';
import type { ApiError } from '@/api/types';
import {
    useCreateTrafficControlRule,
    useDeleteTrafficControlRule,
    useTrafficControlRuleList,
    useUpdateTrafficControlRule,
    type CreateTrafficControlRuleRequest,
    type TrafficControlRule,
} from '@/api/endpoints/traffic-control';

type RuleFormValue = CreateTrafficControlRuleRequest;

const defaultForm: RuleFormValue = {
    name: '',
    description: '',
    enabled: true,
    priority: 100,
    match_type: 'ip',
    match_config: { ips: [] },
    action_type: 'fast_fail',
    action_config: {
        status_code: 429,
        message: 'request blocked by traffic control',
    },
};

function ipsToText(rule?: TrafficControlRule): string {
    return rule?.match_config?.ips?.join('\n') ?? '';
}

function textToIPs(text: string): string[] {
    return text
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean);
}

function ruleToForm(rule?: TrafficControlRule): RuleFormValue {
    if (!rule) return defaultForm;
    return {
        name: rule.name,
        description: rule.description ?? '',
        enabled: rule.enabled,
        priority: rule.priority,
        match_type: rule.match_type,
        match_config: {
            ...rule.match_config,
            ips: rule.match_config?.ips ?? [],
        },
        action_type: rule.action_type,
        action_config: {
            status_code: rule.action_config?.status_code ?? 429,
            message: rule.action_config?.message ?? 'request blocked by traffic control',
            limit: rule.action_config?.limit,
            window_sec: rule.action_config?.window_sec,
        },
    };
}

function getErrorMessage(error: unknown): string | undefined {
    return (error as ApiError | undefined)?.message;
}

function RuleDialog({
    open,
    rule,
    isPending,
    onOpenChange,
    onSubmit,
}: {
    open: boolean;
    rule?: TrafficControlRule;
    isPending: boolean;
    onOpenChange: (open: boolean) => void;
    onSubmit: (data: RuleFormValue) => void;
}) {
    const [form, setForm] = useState<RuleFormValue>(() => ruleToForm(rule));
    const [ipText, setIpText] = useState(() => ipsToText(rule));

    const updateForm = useCallback((patch: Partial<RuleFormValue>) => {
        setForm((prev) => ({ ...prev, ...patch }));
    }, []);

    const updateAction = useCallback((patch: Partial<RuleFormValue['action_config']>) => {
        setForm((prev) => ({ ...prev, action_config: { ...prev.action_config, ...patch } }));
    }, []);

    const handleSubmit = useCallback((event: React.FormEvent) => {
        event.preventDefault();
        const ips = textToIPs(ipText);
        onSubmit({
            ...form,
            match_type: 'ip',
            match_config: { ...form.match_config, ips },
            action_type: 'fast_fail',
        });
    }, [form, ipText, onSubmit]);

    const canSubmit = form.name.trim().length > 0 && textToIPs(ipText).length > 0;

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-2xl rounded-2xl">
                <DialogHeader>
                    <DialogTitle>{rule ? '编辑流量控制规则' : '新增流量控制规则'}</DialogTitle>
                </DialogHeader>
                <form onSubmit={handleSubmit} className="grid gap-4">
                    <div className="grid grid-cols-1 gap-3 md:grid-cols-[1fr_120px]">
                        <label className="grid gap-1 text-sm">
                            名称
                            <Input
                                value={form.name}
                                onChange={(event) => updateForm({ name: event.target.value })}
                                disabled={isPending}
                                required
                            />
                        </label>
                        <label className="grid gap-1 text-sm">
                            优先级
                            <Input
                                type="number"
                                value={form.priority}
                                onChange={(event) => updateForm({ priority: Number(event.target.value) || 100 })}
                                disabled={isPending}
                            />
                        </label>
                    </div>

                    <label className="grid gap-1 text-sm">
                        描述
                        <Input
                            value={form.description ?? ''}
                            onChange={(event) => updateForm({ description: event.target.value })}
                            disabled={isPending}
                        />
                    </label>

                    <label className="grid gap-1 text-sm">
                        IP / IP 段
                        <textarea
                            value={ipText}
                            onChange={(event) => setIpText(event.target.value)}
                            disabled={isPending}
                            placeholder={'192.168.1.10\n10.0.0.0/24\n172.16.0.10-172.16.0.20'}
                            className="min-h-28 rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] disabled:opacity-50"
                            required
                        />
                    </label>

                    <div className="grid grid-cols-1 gap-3 md:grid-cols-[120px_1fr]">
                        <label className="grid gap-1 text-sm">
                            状态码
                            <Input
                                type="number"
                                min={400}
                                max={599}
                                value={form.action_config.status_code ?? 429}
                                onChange={(event) => updateAction({ status_code: Number(event.target.value) || 429 })}
                                disabled={isPending}
                            />
                        </label>
                        <label className="grid gap-1 text-sm">
                            快速失败消息
                            <Input
                                value={form.action_config.message ?? ''}
                                onChange={(event) => updateAction({ message: event.target.value })}
                                disabled={isPending}
                            />
                        </label>
                    </div>

                    <div className="flex items-center justify-between rounded-lg border p-3">
                        <div>
                            <div className="text-sm font-medium">启用规则</div>
                            <div className="text-xs text-muted-foreground">关闭后规则会保留，但不会参与请求匹配。</div>
                        </div>
                        <Switch
                            checked={form.enabled}
                            onCheckedChange={(enabled) => updateForm({ enabled })}
                            disabled={isPending}
                        />
                    </div>

                    <DialogFooter>
                        <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isPending}>
                            <X className="size-4" />
                            取消
                        </Button>
                        <Button type="submit" disabled={isPending || !canSubmit}>
                            {isPending ? <Loader className="size-4 animate-spin" /> : <Check className="size-4" />}
                            保存
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}

export function TrafficControl() {
    const { data: rules = [], isLoading, error } = useTrafficControlRuleList();
    const createRule = useCreateTrafficControlRule();
    const updateRule = useUpdateTrafficControlRule();
    const deleteRule = useDeleteTrafficControlRule();
    const [editingRule, setEditingRule] = useState<TrafficControlRule | null>(null);
    const [isCreating, setIsCreating] = useState(false);

    const sortedRules = useMemo(() => [...rules].sort((a, b) => a.priority - b.priority || a.id - b.id), [rules]);
    const isMutating = createRule.isPending || updateRule.isPending;

    const handleCreate = useCallback((data: RuleFormValue) => {
        createRule.mutate(data, {
            onSuccess: () => {
                toast.success('流量控制规则已创建');
                setIsCreating(false);
            },
            onError: (err) => toast.error('创建失败', { description: getErrorMessage(err) }),
        });
    }, [createRule]);

    const handleUpdate = useCallback((data: RuleFormValue) => {
        if (!editingRule) return;
        updateRule.mutate({ id: editingRule.id, ...data }, {
            onSuccess: () => {
                toast.success('流量控制规则已更新');
                setEditingRule(null);
            },
            onError: (err) => toast.error('更新失败', { description: getErrorMessage(err) }),
        });
    }, [editingRule, updateRule]);

    const handleDelete = useCallback((id: number) => {
        deleteRule.mutate(id, {
            onSuccess: () => toast.success('流量控制规则已删除'),
            onError: (err) => toast.error('删除失败', { description: getErrorMessage(err) }),
        });
    }, [deleteRule]);

    return (
        <PageWrapper className="space-y-4">
            <section className="flex flex-col gap-3 rounded-lg border bg-card p-5 md:flex-row md:items-center md:justify-between">
                <div className="flex items-center gap-3">
                    <div className="flex size-10 items-center justify-center rounded-lg bg-destructive/10 text-destructive">
                        <ShieldAlert className="size-5" />
                    </div>
                    <div>
                        <h1 className="text-xl font-semibold">流量控制</h1>
                        <p className="text-sm text-muted-foreground">当前支持按 IP、CIDR 或起止 IP 段快速失败。</p>
                    </div>
                </div>
                <Button onClick={() => setIsCreating(true)}>
                    <Plus className="size-4" />
                    新增规则
                </Button>
            </section>

            <section className="rounded-lg border bg-card">
                {isLoading ? (
                    <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
                        <Loader className="mr-2 size-4 animate-spin" />
                        加载中
                    </div>
                ) : error ? (
                    <div className="flex h-40 items-center justify-center text-sm text-destructive">加载失败</div>
                ) : sortedRules.length === 0 ? (
                    <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">暂无流量控制规则</div>
                ) : (
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>规则</TableHead>
                                <TableHead>IP / IP 段</TableHead>
                                <TableHead>动作</TableHead>
                                <TableHead>优先级</TableHead>
                                <TableHead>状态</TableHead>
                                <TableHead className="w-28 text-right">操作</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {sortedRules.map((rule) => (
                                <TableRow key={rule.id}>
                                    <TableCell>
                                        <div className="font-medium">{rule.name}</div>
                                        {rule.description && (
                                            <div className="text-xs text-muted-foreground">{rule.description}</div>
                                        )}
                                    </TableCell>
                                    <TableCell>
                                        <div className="max-w-[360px] truncate font-mono text-xs">
                                            {(rule.match_config.ips ?? []).join(', ')}
                                        </div>
                                    </TableCell>
                                    <TableCell>
                                        <div className="flex items-center gap-2 text-sm">
                                            <Ban className="size-4 text-destructive" />
                                            快速失败 {rule.action_config.status_code ?? 429}
                                        </div>
                                    </TableCell>
                                    <TableCell>{rule.priority}</TableCell>
                                    <TableCell>
                                        <span className={rule.enabled ? 'text-emerald-600' : 'text-muted-foreground'}>
                                            {rule.enabled ? '启用' : '停用'}
                                        </span>
                                    </TableCell>
                                    <TableCell>
                                        <div className="flex justify-end gap-1">
                                            <Button variant="ghost" size="icon" onClick={() => setEditingRule(rule)}>
                                                <Pencil className="size-4" />
                                            </Button>
                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                onClick={() => handleDelete(rule.id)}
                                                disabled={deleteRule.isPending}
                                                className="text-destructive hover:text-destructive"
                                            >
                                                <Trash2 className="size-4" />
                                            </Button>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}
            </section>

            {isCreating && (
                <RuleDialog
                    key="create"
                    open={isCreating}
                    isPending={isMutating}
                    onOpenChange={setIsCreating}
                    onSubmit={handleCreate}
                />
            )}
            {editingRule && (
                <RuleDialog
                    key={editingRule.id}
                    open={!!editingRule}
                    rule={editingRule}
                    isPending={isMutating}
                    onOpenChange={(open) => !open && setEditingRule(null)}
                    onSubmit={handleUpdate}
                />
            )}
        </PageWrapper>
    );
}
