"use client";

import { useCallback, useMemo, useState } from "react";
import { useClearLogs, useLogs } from "@/api/endpoints/log";
import { LogCard } from "./Item";
import { Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { VirtualizedGrid } from "@/components/common/VirtualizedGrid";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/common/Toast";

/**
 * 日志页面组件
 * - 初始加载 pageSize 条历史日志
 * - SSE 实时推送新日志
 * - 滚动自动加载更多
 */
export function Log() {
  const tt = useTranslations("setting");
  const t = useTranslations("log");
  const { logs, hasMore, isLoading, isLoadingMore, loadMore } = useLogs({
    pageSize: 10,
  });

  const [isClearing, setIsClearing] = useState(false);
  const clearLogs = useClearLogs();
  const handleClearLogs = () => {
    setIsClearing(true);
    clearLogs.mutate(undefined, {
      onSuccess: () => {
        toast.success(tt("log.clearSuccess"));
        setIsClearing(false);
      },
      onError: () => {
        toast.error(tt("log.clearFailed"));
        setIsClearing(false);
      },
    });
  };

  const canLoadMore =
    hasMore && !isLoading && !isLoadingMore && logs.length > 0;
  const handleReachEnd = useCallback(() => {
    if (!canLoadMore) return;
    void loadMore();
  }, [canLoadMore, loadMore]);

  const footer = useMemo(() => {
    if (hasMore && (isLoading || isLoadingMore)) {
      return (
        <div className="flex justify-center py-4">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      );
    }
    if (!hasMore && logs.length > 0) {
      return (
        <div className="flex justify-center py-4">
          <span className="text-sm text-muted-foreground">
            {t("list.noMore")}
          </span>
        </div>
      );
    }
    return null;
  }, [hasMore, isLoading, isLoadingMore, logs.length, t]);

  return (
    <>
      <Button
        variant="destructive"
        size="sm"
        onClick={handleClearLogs}
        disabled={isClearing}
        className="rounded-xl"
      >
        {isClearing ? tt("log.clear.clearing") : tt("log.clear.button")}
      </Button>
      <VirtualizedGrid
        items={logs}
        layout="list"
        columns={{ default: 1 }}
        estimateItemHeight={80}
        overscan={8}
        getItemKey={(log) => `log-${log.id}`}
        renderItem={(log) => <LogCard log={log} />}
        footer={footer}
        onReachEnd={handleReachEnd}
        reachEndEnabled={canLoadMore}
        reachEndOffset={2}
      />
    </>
  );
}
