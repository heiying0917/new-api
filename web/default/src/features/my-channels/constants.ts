/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

// Re-export channel type options from the channels feature for reuse
export {
  CHANNEL_TYPE_OPTIONS,
  CHANNEL_TYPES,
  CHANNEL_STATUS,
  CHANNEL_STATUS_CONFIG,
} from '@/features/channels/constants'

export const MY_CHANNEL_STATUS_CONFIG = {
  0: { variant: 'neutral' as const, label: 'Unknown' },
  1: { variant: 'success' as const, label: 'Enabled' },
  2: { variant: 'danger' as const, label: 'Disabled' },
  3: { variant: 'warning' as const, label: 'Auto Disabled' },
} as const

export function getChannelTypeLabel(type: number): string {
  // Import lazily to avoid circular; the CHANNEL_TYPES object is simple
  const map: Record<number, string> = {
    0: 'Unknown',
    1: 'OpenAI',
    2: 'Midjourney',
    3: 'Azure',
    4: 'Ollama',
    5: 'MidjourneyPlus',
    6: 'OpenAIMax',
    7: 'OhMyGPT',
    8: 'Custom',
    9: 'AILS',
    10: 'AI Proxy',
    11: 'PaLM',
    12: 'API2GPT',
    13: 'AIGC2D',
    14: 'Anthropic',
    15: 'Baidu',
    16: 'Zhipu',
    17: 'Ali',
    18: 'Xunfei',
    19: '360',
    20: 'OpenRouter',
    21: 'AI Proxy Library',
    22: 'FastGPT',
    23: 'Tencent',
    24: 'Gemini',
    25: 'Moonshot',
    26: 'Zhipu V4',
    27: 'Perplexity',
    31: 'LingYiWanWu',
    33: 'AWS',
    34: 'Cohere',
    35: 'MiniMax',
    36: 'SunoAPI',
    37: 'Dify',
    38: 'Jina',
    39: 'Cloudflare',
    40: 'SiliconFlow',
    41: 'Vertex AI',
    42: 'Mistral',
    43: 'DeepSeek',
    44: 'MokaAI',
    45: 'VolcEngine',
    46: 'Baidu V2',
    47: 'Xinference',
    48: 'xAI',
    49: 'Coze',
    50: 'Kling',
    51: 'Jimeng',
    52: 'Vidu',
    53: 'Submodel',
    54: 'DoubaoVideo',
    55: 'Sora',
    56: 'Replicate',
    57: 'Codex',
  }
  return map[type] ?? String(type)
}
