// Policy templates. A template is a whole starting policy: a name, the question, the routes with
// the tier of model each one needs, and the kind of decision model that classifies it best.
// Applying one fills the targets, the fallback and the decision model from the models the policy
// can use. Titles and summaries are in routingPolicy.templates.<id>.*; questions, labels and
// descriptions stay in English because the decision model reads them.

// Model tiers a route asks for. Each tier lists name patterns in order of preference; the first
// pattern that matches an available model wins, and among its matches the newest version.
export const TIERS = ['fast', 'balanced', 'strongest'];

const TIER_PATTERNS = {
  fast: [
    /claude-haiku/,
    /gemini-[\d.]+-flash(?!-lite)/,
    /deepseek-.*flash/,
    /gpt-[\d.]+-mini/,
    /gemini-.*flash-lite/,
    /gpt-.*nano/,
    /(haiku|flash|mini|lite|nano)/
  ],
  balanced: [/claude-sonnet/, /gemini-[\d.]+-pro/, /deepseek-.*-pro/, /(sonnet|-pro)/],
  strongest: [/claude-opus/, /claude-fable/, /opus/]
};

// Tiers to try when a tier has no model: the nearest one first.
const TIER_FALLBACK = {
  fast: ['fast', 'balanced', 'strongest'],
  balanced: ['balanced', 'strongest', 'fast'],
  strongest: ['strongest', 'balanced', 'fast']
};

// Decision models in order of preference. Coding prompts: Jev, the strongest on code in published comparisons;
// everything else: a general decision model, the cheapest fast one next.
export const DECISION_MODELS = {
  coding: ['jev-1.14.0', 'jev', 'jev-1.13.0', 'gemini-decision-1', 'pplx-decider-flash-v1.3', 'd1'],
  general: ['gemini-decision-1', 'pplx-decider-flash-v1.3', 'pplx-decider-v1.3', 'd1', 'jev-1.14.0', 'jev']
};

// Recommended decision models in the order the picker lists them.
export const RECOMMENDED_DECISION_MODELS = [...new Set([...DECISION_MODELS.coding, ...DECISION_MODELS.general])];

// A policy has 2 to 16 routes.
export const MIN_ROUTES = 2;
export const MAX_ROUTES = 16;
// A fallback policy has 2 to 8 models.
export const MIN_FALLBACK_MODELS = 2;
export const MAX_FALLBACK_MODELS = 8;

// suggestFallbackChain: a starting chain for a new fallback policy, the strongest model first and
// the balanced one behind it; models already chosen are kept.
export function suggestFallbackChain(models = []) {
  const chain = [pickTierModel(models, 'strongest'), pickTierModel(models, 'balanced')].filter(Boolean);
  return [...new Set(chain)];
}

export const POLICY_TEMPLATES = [
  {
    // How much of the codebase a change touches. Additive work following an existing pattern is
    // cheap; behaviour that must change consistently in several places needs the strongest model.
    id: 'coding_change',
    name: 'coder',
    decision: 'coding',
    question:
      'This is the first message a developer sent to a coding agent working in an existing codebase. What kind of change is it? If the request lists several existing places that must all change, prefer the later label.',
    routes: [
      {
        label: 'additive',
        tier: 'fast',
        description: 'Add something new that follows an existing pattern: a field, a constant, a helper, an adapter or an endpoint'
      },
      {
        label: 'local',
        tier: 'balanced',
        description: 'Change existing behaviour in one place: one function, one handler or one component'
      },
      {
        label: 'cross_cutting',
        tier: 'strongest',
        description:
          'Change existing behaviour in several places that must all stay consistent: handlers, protocol directions, streaming paths, client and server'
      },
      { label: 'refactor', tier: 'strongest', description: 'Rename, move or restructure code without changing what it does' }
    ]
  },
  {
    // What a coding agent is asked to do: reading and answering is cheap, planning a larger piece
    // of work needs the strongest model.
    id: 'agent_task',
    name: 'agent',
    decision: 'coding',
    question: 'This is the first thing a developer typed into a coding agent. What do they want the agent to do?',
    routes: [
      { label: 'explain', tier: 'fast', description: 'Read code or docs and answer a question about them; nothing is edited' },
      {
        label: 'edit',
        tier: 'balanced',
        description: 'Make a concrete, already decided change: add a flag, rename a field, write a test, update a config'
      },
      {
        label: 'debug',
        tier: 'balanced',
        description: 'Find out why something fails: an error message, a failing test, a crash or wrong output'
      },
      {
        label: 'plan',
        tier: 'strongest',
        description: 'Design or carry out a larger piece of work over many files: a new feature, a migration, an architecture change'
      }
    ]
  },
  {
    // How much thinking a request needs: chit-chat and lookups stay fast, multi-step reasoning
    // escalates.
    id: 'reasoning_depth',
    name: 'general',
    decision: 'general',
    question: 'How much thinking does it take to answer this request well?',
    routes: [
      { label: 'quick', tier: 'fast', description: 'Small talk, a greeting, a fact lookup or a short answer that needs no reasoning' },
      { label: 'explain', tier: 'balanced', description: 'Explain, compare or summarise something in a few paragraphs' },
      {
        label: 'deep',
        tier: 'strongest',
        description: 'Multi-step reasoning: math, logic puzzles, planning, weighing trade-offs or a decision with many constraints'
      }
    ]
  },
  {
    // One general model name for a team: code goes to a coding model, prose to a writing model,
    // numbers and tables to a model that is good at analysis.
    id: 'content_type',
    name: 'team',
    decision: 'general',
    question: 'What is the main material of this request?',
    routes: [
      { label: 'chat', tier: 'fast', description: 'Small talk or a short everyday question that needs no special skill' },
      {
        label: 'writing',
        tier: 'balanced',
        description: 'Drafting, rewriting, translating or summarising text such as an email, an article or a report'
      },
      { label: 'code', tier: 'strongest', description: 'Source code, shell commands, SQL or configuration files' },
      { label: 'data', tier: 'balanced', description: 'Numbers, tables, spreadsheets or calculations to analyse' }
    ]
  },
  {
    // Writing work: translation and light edits are cheap, long or creative pieces are not.
    id: 'writing_task',
    name: 'writer',
    decision: 'general',
    question: 'This is a request to a writing assistant. What kind of writing work is it?',
    routes: [
      { label: 'translate', tier: 'fast', description: 'Translate a given text into another language' },
      { label: 'polish', tier: 'fast', description: 'Fix grammar, shorten or change the tone of a text that is already written' },
      { label: 'draft', tier: 'balanced', description: 'Write a new email, post, document or report from notes or a brief' },
      {
        label: 'creative',
        tier: 'strongest',
        description: 'Long or creative work: a story, a speech, a campaign, or a piece that must persuade'
      }
    ]
  },
  {
    // Customer support: simple account questions stay cheap, real problems and upset customers
    // go to a careful model.
    id: 'support_triage',
    name: 'support',
    decision: 'general',
    question: 'This is the first message a customer sent to our support assistant. What does the customer need?',
    routes: [
      {
        label: 'account',
        tier: 'fast',
        description: 'A simple question about their account, plan, invoice, password or settings that has a known answer'
      },
      { label: 'how_to', tier: 'fast', description: 'Asks how to do something with the product, step by step' },
      {
        label: 'problem',
        tier: 'balanced',
        description: 'Reports something that does not work: an error, missing data or unexpected behaviour that needs troubleshooting'
      },
      {
        label: 'escalation',
        tier: 'strongest',
        description: 'Upset or threatening to leave, asks for a refund or compensation, or raises a legal, security or privacy concern'
      }
    ]
  },
  {
    // Requests where a wrong answer is costly go to the most careful model, the rest stay cheap.
    id: 'stakes',
    name: 'careful',
    decision: 'general',
    question: 'How costly would a wrong or careless answer to this request be?',
    routes: [
      { label: 'routine', tier: 'fast', description: 'Everyday work where a small mistake is easy to notice and fix' },
      {
        label: 'careful',
        tier: 'strongest',
        description: 'Legal, medical, financial or security matters, or anything that will be sent to customers as is'
      }
    ]
  }
];

// Variants of a model (batch pricing, fast or preview tiers, dated snapshots) are never picked for
// a template: the plain model is the one to start from.
const VARIANT = /-(batch|fast|ultrafast|highspeed|lightning|ultraspeed|preview|exp|live|realtime|vision|vl|\d{4,})(-|$)/;
const newestFirst = (a, b) => b.localeCompare(a, undefined, { numeric: true });

// pickTierModel is the model of `models` that best fills `tier`, trying the nearest tiers when it
// has none; '' when no model matches any tier.
export function pickTierModel(models = [], tier = 'balanced') {
  for (const t of TIER_FALLBACK[tier] || TIER_FALLBACK.balanced) {
    for (const pattern of TIER_PATTERNS[t]) {
      const matches = models.filter((m) => !VARIANT.test(m.toLowerCase()) && pattern.test(m.toLowerCase())).sort(newestFirst);
      if (matches.length) return matches[0];
    }
  }
  return '';
}

// pickDecisionModel: the first preferred decision model that is available, any other decision
// model next, a fast chat model last.
export function pickDecisionModel(template, classifiers = {}) {
  const decision = classifiers.decision || [];
  const preferred = DECISION_MODELS[template?.decision] || DECISION_MODELS.general;
  return preferred.find((m) => decision.includes(m)) || decision[0] || pickTierModel(classifiers.chat || [], 'fast');
}

// applyTemplate merges a template into a form: the question and routes are replaced, a target
// already chosen keeps its position, empty targets get the model of the route's tier, and an
// empty decision model or fallback is filled (the fallback with the strongest model).
export function applyTemplate(form, template, { models = [], classifiers } = {}) {
  const routes = template.routes.map(({ label, description, tier }, index) => ({
    label,
    description,
    target_model: form.routes?.[index]?.target_model || pickTierModel(models, tier)
  }));
  return {
    ...form,
    question: template.question,
    routes,
    decision_model: form.decision_model || pickDecisionModel(template, classifiers || { chat: models }),
    fallback_model: form.fallback_model || pickTierModel(models, 'strongest')
  };
}

// templateName is the model name a template suggests: auto/<name> for platform policies, which
// share the namespace with every model, and the bare name where clients call policy/<name>.
export function templateName(template, callPrefix) {
  return callPrefix ? template.name : `auto/${template.name}`;
}
