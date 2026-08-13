<template>
    <div v-if="status" class="memory-status-bar" :class="{ 'is-attention': status.attention }">
        <div class="memory-status-metrics">
            <span class="memory-status-item memory-status-tokens"
                :title="`模型上下文窗口 ${formatTokens(status.context_window)} token，当前约 ${formatTokens(status.current_tokens)}（估算）`">
                上下文约 {{ formatTokens(status.current_tokens) }} / {{ formatTokens(status.token_threshold) }} token
                ({{ tokenPercent }}%)
            </span>
            <span v-if="status.turns_threshold > 0" class="memory-status-item memory-status-turns"
                :title="`距上次压缩 ${status.current_turns} 轮，达到 ${status.turns_threshold} 轮自动压缩`">
                历史 {{ status.current_turns }}/{{ status.turns_threshold }} 轮
            </span>
            <span v-if="status.compressed_count > 0" class="memory-status-item memory-status-compressed"
                :title="status.summary_type === 'raw' ? 'LLM 摘要不可用，已降级为原文归档' : '早期对话已合并进摘要'">
                {{ status.summary_type === 'raw' ? '原文归档' : '已压缩' }} {{ status.compressed_count }} 条
            </span>
            <span v-if="status.attention" class="memory-status-item memory-status-hint">
                {{ status.current_tokens > status.token_threshold ? '上下文占用已达阈值' : '历史轮数已达阈值' }}
            </span>
        </div>
        <div class="memory-status-track" :title="`token 占用 ${tokenPercent}%（约 ${formatTokens(status.current_tokens)}/${formatTokens(status.token_threshold)}）`">
            <div class="memory-status-fill" :class="{ 'is-attention': status.attention }"
                :style="{ width: tokenPercent + '%' }"></div>
        </div>
        <t-button size="small" variant="outline" theme="default" class="memory-status-btn"
            :loading="compressing" :disabled="compressing" @click="onCompress">
            {{ compressing ? '压缩中' : '压缩上下文' }}
        </t-button>
    </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { MessagePlugin } from 'tdesign-vue-next';
import { getMemoryStatus, compressMemory, type MemoryStatus } from '@/api/chat/memory';

const props = defineProps<{
    sessionId: string;
}>();

const status = ref<MemoryStatus | null>(null);
const loading = ref(false);
const compressing = ref(false);
let fetchSeq = 0; // 会话切换竞态防护：丢弃过期响应

const tokenPercent = computed(() => {
    if (!status.value || status.value.token_threshold <= 0) return 0;
    return Math.min(100, Math.round((status.value.current_tokens / status.value.token_threshold) * 100));
});

const formatTokens = (n: number) => {
    if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
    if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
    return String(n);
};

// 拉取状态：会话切换（sessionId 变化）或外部显式调用 refresh()
const fetchStatus = async () => {
    if (!props.sessionId || props.sessionId === 'undefined') return;
    const seq = ++fetchSeq;
    loading.value = true;
    try {
        const res = await getMemoryStatus(props.sessionId);
        if (seq !== fetchSeq) return; // 已切换到其他会话，丢弃过期响应
        status.value = res?.data ?? null;
    } catch (err) {
        if (seq !== fetchSeq) return;
        status.value = null;
        console.warn('[MemoryStatus] 获取上下文状态失败:', err);
    } finally {
        if (seq === fetchSeq) loading.value = false;
    }
};

const refresh = () => { fetchStatus(); };

const onCompress = async () => {
    if (!props.sessionId || compressing.value) return;
    compressing.value = true;
    try {
        const res = await compressMemory(props.sessionId);
        const newStatus = res?.data ?? null;
        status.value = newStatus;
        const compressed = newStatus?.last_compressed_count || 0;
        if (compressed > 0) {
            MessagePlugin.success(`已压缩 ${compressed} 条历史消息`);
        } else {
            MessagePlugin.info('历史较短，暂无需压缩');
        }
    } catch (err: any) {
        const msg = err?.message || err?.data?.message || '压缩失败';
        if (err?.data?.code === 409) {
            MessagePlugin.warning(msg);
        } else {
            MessagePlugin.error(msg);
        }
    } finally {
        compressing.value = false;
    }
};

watch(() => props.sessionId, () => { fetchStatus(); });

onMounted(() => { fetchStatus(); });
onBeforeUnmount(() => { fetchSeq++; });

defineExpose({ refresh });
</script>

<style scoped>
.memory-status-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 12px;
    margin-bottom: 6px;
    border: 1px solid var(--td-component-stroke, #e7e7e7);
    border-radius: var(--td-radius-default, 6px);
    background: var(--td-bg-color-container, #fff);
    font-size: 12px;
    color: var(--td-text-color-secondary, #5a5a5a);
    transition: border-color 0.2s;
}

.memory-status-bar.is-attention {
    border-color: var(--td-warning-color, #ed7b2f);
    background: var(--td-warning-color-1, #fff4e8);
}

.memory-status-metrics {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
    flex: 1;
    min-width: 0;
}

.memory-status-item {
    white-space: nowrap;
}

.memory-status-hint {
    color: var(--td-warning-color, #ed7b2f);
    font-weight: 500;
}

.memory-status-track {
    flex: 0 1 160px;
    height: 4px;
    border-radius: 2px;
    background: var(--td-bg-color-component, #f3f3f3);
    overflow: hidden;
    min-width: 60px;
}

.memory-status-fill {
    height: 100%;
    border-radius: 2px;
    background: var(--td-brand-color, #0052d9);
    transition: width 0.3s;
}

.memory-status-fill.is-attention {
    background: var(--td-warning-color, #ed7b2f);
}

.memory-status-btn {
    flex-shrink: 0;
}
</style>
