// ==============================|| API CATALOG — DECISION ||============================== //
// 「决策」模态的能力目录。端点对照 router/relay-router.go:
//   setOpenAIRouter -> POST /v1/decisions(OpenAI Decisions API,relay/decisions.go)
// 决策模型不生成文本:对同一份输入问一组带类型的问题(predicate / choice / score),
// 返回概率、选项与分数,只按输入 token 计费。没有 modality:价格页按输出模态分 Tab,
// 决策模型输出不是一种模态,所以本页不嵌价格表,可用模型由能力节按 endpoints 列出。

import { curlOpenAI, jsClient, pyClient, pyLiteral } from './_helpers';

const TICKET = 'The package arrived with a broken screen. I want my money back.';

const QUESTIONS = [
  { type: 'predicate', name: 'damaged', instructions: 'Does the customer report a damaged item?' },
  {
    type: 'choice',
    name: 'queue',
    instructions: 'Which team should handle this ticket?',
    choices: [
      { value: 'billing', description: 'Refunds and payments' },
      { value: 'shipping', description: 'Delivery and damaged items' }
    ]
  },
  {
    type: 'score',
    name: 'urgency',
    instructions: 'How urgent is this ticket?',
    levels: [{ label: 'low' }, { label: 'medium' }, { label: 'high' }]
  }
];

const IMAGE_PLACEHOLDER = 'data:image/png;base64,BASE64_IMAGE';
const IMAGE_FILE = 'photo.png';

const imageInput = (imageUrl) => [
  {
    role: 'user',
    content: [
      { type: 'input_text', text: 'Inspect the product in this photo.' },
      { type: 'input_image', image_url: imageUrl, detail: 'high' }
    ]
  }
];

const IMAGE_QUESTIONS = [{ type: 'predicate', name: 'visible_damage', instructions: 'Does the product have visible damage?' }];

const jsLiteral = (value) => JSON.stringify(value, null, 2).split('\n').join('\n  ');

const decision = {
  key: 'decision',
  titleKey: 'api_decision',
  apiNameKey: 'apiCatalogPage.decision.apiName',
  introKey: 'apiCatalogPage.decision.intro',
  docsPath: '/guide/decisions',
  capabilities: [
    {
      id: 'questions',
      titleKey: 'apiCatalogPage.decision.capabilities.questions.title',
      descKey: 'apiCatalogPage.decision.capabilities.questions.desc',
      noteKey: 'apiCatalogPage.decision.capabilities.questions.note',
      endpoint: { method: 'POST', path: '/v1/decisions' },
      transport: 'decisions',
      availability: { endpoints: ['decisions'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) => curlOpenAI(baseUrl, apiKey, '/v1/decisions', { model, input: TICKET, questions: QUESTIONS }),
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
decision = client.decisions.create(
    model="${model}",
    input="${TICKET}",
    questions=${pyLiteral(QUESTIONS, 1)},
)

for answer in decision.answers:
    print(answer)`,
        javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const decision = await client.decisions.create({
  model: '${model}',
  input: '${TICKET}',
  questions: ${jsLiteral(QUESTIONS)}
});

console.log(decision.answers);`
      }
    },
    {
      id: 'image',
      titleKey: 'apiCatalogPage.decision.capabilities.image.title',
      descKey: 'apiCatalogPage.decision.capabilities.image.desc',
      noteKey: 'apiCatalogPage.decision.capabilities.image.note',
      endpoint: { method: 'POST', path: '/v1/decisions' },
      transport: 'decisions',
      availability: { endpoints: ['decisions'], inputModalities: ['image'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/decisions', { model, input: imageInput(IMAGE_PLACEHOLDER), questions: IMAGE_QUESTIONS }),
        python: ({ baseUrl, apiKey, model }) => `import base64

${pyClient(baseUrl, apiKey)}
image = base64.b64encode(open("${IMAGE_FILE}", "rb").read()).decode()

decision = client.decisions.create(
    model="${model}",
    input=[
        {
            "role": "user",
            "content": [
                {"type": "input_text", "text": "Inspect the product in this photo."},
                {"type": "input_image", "image_url": f"data:image/png;base64,{image}", "detail": "high"},
            ],
        }
    ],
    questions=${pyLiteral(IMAGE_QUESTIONS, 1)},
)
print(decision.answers[0])`,
        javascript: ({ baseUrl, apiKey, model }) => `import fs from 'node:fs';
${jsClient(baseUrl, apiKey)}
const image = fs.readFileSync('${IMAGE_FILE}').toString('base64');

const decision = await client.decisions.create({
  model: '${model}',
  input: [
    {
      role: 'user',
      content: [
        { type: 'input_text', text: 'Inspect the product in this photo.' },
        { type: 'input_image', image_url: \`data:image/png;base64,\${image}\`, detail: 'high' }
      ]
    }
  ],
  questions: ${jsLiteral(IMAGE_QUESTIONS)}
});

console.log(decision.answers[0]);`
      }
    }
  ]
};

export default decision;
