import { describe, expect, it } from 'vitest';

import { typeConfig } from './typeConfig';

describe('typeConfig', () => {
  it('shows the fetch-models button only for types whose backend can list models', () => {
    // Types whose provider implements GetModelList against an existing upstream endpoint. Left
    // out on purpose: Baidu, Baichuan, MiniMax, 01.AI, Suno and Jina (no list URL configured),
    // Gemini CLI, Antigravity and Vertex AI Express (the Gemini /v1beta/models path does not
    // exist on their endpoints), Claude Code (the list call lacks the OAuth beta header) and
    // Codex (/backend-api/models does not return the OpenAI list shape).
    const withButton = Object.keys(typeConfig)
      .filter((id) => typeConfig[id].inputLabel?.provider_models_list)
      .map(Number)
      .sort((a, b) => a - b);
    expect(withButton).toEqual([1, 3, 8, 14, 17, 20, 25, 28, 29, 30, 31, 36, 39, 45, 49, 55, 56, 62]);
  });

  it('picks a test model from the default models', () => {
    for (const [id, cfg] of Object.entries(typeConfig)) {
      const { models, test_model: testModel } = cfg.input || {};
      if (testModel) expect(models, `type ${id}`).toContain(testModel);
    }
  });

  it('has no duplicate default models', () => {
    for (const [id, cfg] of Object.entries(typeConfig)) {
      const models = cfg.input?.models || [];
      expect(new Set(models).size, `type ${id}`).toBe(models.length);
    }
  });
});
