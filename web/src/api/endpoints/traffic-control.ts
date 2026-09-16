import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';
import { logger } from '@/lib/logger';

export type TrafficControlMatchType = 'ip' | 'path' | 'body' | 'header' | 'composite';
export type TrafficControlActionType = 'fast_fail' | 'concurrency';

export interface TrafficControlMatchConfig {
    ips?: string[];
    paths?: string[];
    headers?: string[];
    body?: string;
    mode?: string;
}

export interface TrafficControlActionConfig {
    status_code?: number;
    message?: string;
    limit?: number;
    window_sec?: number;
}

export interface TrafficControlRule {
    id: number;
    name: string;
    description?: string;
    enabled: boolean;
    priority: number;
    match_type: TrafficControlMatchType;
    match_config: TrafficControlMatchConfig;
    action_type: TrafficControlActionType;
    action_config: TrafficControlActionConfig;
    created_at?: string;
    updated_at?: string;
}

export type CreateTrafficControlRuleRequest = Omit<TrafficControlRule, 'id' | 'created_at' | 'updated_at'>;
export type UpdateTrafficControlRuleRequest = Pick<TrafficControlRule, 'id'> & CreateTrafficControlRuleRequest;

export function useTrafficControlRuleList() {
    return useQuery({
        queryKey: ['traffic-control', 'list'],
        queryFn: () => apiClient.get<TrafficControlRule[]>('/api/v1/traffic-control/list'),
        refetchInterval: 30000,
    });
}

export function useCreateTrafficControlRule() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (data: CreateTrafficControlRuleRequest) =>
            apiClient.post<TrafficControlRule>('/api/v1/traffic-control/create', data),
        onSuccess: (data) => {
            logger.log('Traffic control rule created:', data);
            queryClient.invalidateQueries({ queryKey: ['traffic-control', 'list'] });
        },
        onError: (error) => {
            logger.error('Traffic control rule create failed:', error);
        },
    });
}

export function useUpdateTrafficControlRule() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (data: UpdateTrafficControlRuleRequest) =>
            apiClient.post<TrafficControlRule>('/api/v1/traffic-control/update', data),
        onSuccess: (data) => {
            logger.log('Traffic control rule updated:', data);
            queryClient.invalidateQueries({ queryKey: ['traffic-control', 'list'] });
        },
        onError: (error) => {
            logger.error('Traffic control rule update failed:', error);
        },
    });
}

export function useDeleteTrafficControlRule() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (id: number) => apiClient.delete<null>(`/api/v1/traffic-control/delete/${id}`),
        onSuccess: () => {
            logger.log('Traffic control rule deleted');
            queryClient.invalidateQueries({ queryKey: ['traffic-control', 'list'] });
        },
        onError: (error) => {
            logger.error('Traffic control rule delete failed:', error);
        },
    });
}
