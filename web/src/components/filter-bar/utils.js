// ==============================|| FILTER BAR — STATE / PARAM HELPERS ||============================== //
// 可复用级联筛选(OpenRouter 风格)的纯逻辑层:字段定义驱动 + 受控状态。
// 值状态形如 { [field.key]: entry }:
//   enum 字段(多选)  -> { op: 'in' | 'not_in', values: string[] }
//   enum 字段(single) -> { op: 'in', values: [string] }(单值,落标量参数)
//   number / text 字段 -> { value: string }
// URL 与 API 参数统一走字段定义上的 paramInclude / paramExclude(enum)或 key(number/text),
// 二者同名以便刷新恢复与请求发送共用一套映射(见 Log 页 fetch/URL 同步)。
// single enum 只用 paramInclude(标量),不支持 exclude;text 为单行标量(后端多为 LIKE/精确)。

export const OP_IN = 'in';
export const OP_NOT_IN = 'not_in';

// 标量值字段(number / text)统一按 entry.value 判活/取值。
const isScalarField = (field) => field.type === 'number' || field.type === 'text';

// 判断单个字段当前是否有有效筛选值(空数组/空串视为未启用,兼容旧链路)。
export function isEntryActive(field, entry) {
  if (!entry) return false;
  if (isScalarField(field)) return String(entry.value ?? '').trim() !== '';
  return Array.isArray(entry.values) && entry.values.length > 0;
}

// 已启用筛选数量(用于筛选按钮上的计数徽标)。
export function countActiveFilters(fields, state) {
  return fields.filter((f) => isEntryActive(f, state[f.key])).length;
}

// 状态 -> 扁平参数对象(enum 为数组,number 为标量字符串)。
// enum 依 op 落到 paramInclude(IN)或 paramExclude(NOT IN)。
export function filterStateToParams(fields, state) {
  const params = {};
  for (const f of fields) {
    const entry = state[f.key];
    if (!isEntryActive(f, entry)) continue;
    if (isScalarField(f)) {
      params[f.key] = String(entry.value).trim();
    } else if (f.single) {
      // 单选 enum:后端只认单值标量(如渠道 type/status/group),落 paramInclude(通常即字段键)。
      params[f.paramInclude] = entry.values[0];
    } else {
      const key = entry.op === OP_NOT_IN ? f.paramExclude : f.paramInclude;
      params[key] = entry.values.slice();
    }
  }
  return params;
}

// URLSearchParams / 查询串 -> 状态(刷新、深链恢复)。exclude 优先于 include。
export function paramsToFilterState(fields, searchParams) {
  const state = {};
  const getAll = (k) => (typeof searchParams.getAll === 'function' ? searchParams.getAll(k) : []);
  for (const f of fields) {
    if (isScalarField(f)) {
      const v = searchParams.get(f.key);
      if (v != null && String(v).trim() !== '') state[f.key] = { value: String(v) };
      continue;
    }
    if (f.single) {
      const v = searchParams.get(f.paramInclude);
      if (v != null && String(v).trim() !== '') state[f.key] = { op: OP_IN, values: [String(v)] };
      continue;
    }
    const inc = getAll(f.paramInclude).filter((x) => x !== '');
    const exc = getAll(f.paramExclude).filter((x) => x !== '');
    if (exc.length) state[f.key] = { op: OP_NOT_IN, values: exc };
    else if (inc.length) state[f.key] = { op: OP_IN, values: inc };
  }
  return state;
}

// 参数对象(可含数组值)-> URLSearchParams,数组以重复键展开(gin 切片绑定所需,不用 [] 后缀)。
export function toSearchParams(obj) {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(obj || {})) {
    if (v == null) continue;
    if (Array.isArray(v)) {
      v.forEach((x) => {
        if (x != null && String(x).trim() !== '') sp.append(k, String(x));
      });
    } else if (String(v).trim() !== '') {
      sp.append(k, String(v));
    }
  }
  return sp;
}

// 单字段 enum 条目的默认值(新建时用)。
export function emptyEnumEntry(op = OP_IN) {
  return { op, values: [] };
}

// 取字段展示用的候选项 label(优先 i18n labelKey,回退 label / value 原文)。
export function optionLabel(t, opt) {
  if (!opt) return '';
  if (opt.labelKey) return t(opt.labelKey);
  return opt.label != null ? opt.label : String(opt.value);
}
