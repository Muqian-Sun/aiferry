// 分组上只剩「允许生图 / 批量生图」开关与批量折扣；单价在模型目录条目上。
export const imagePricingPlatforms = new Set([
  "antigravity",
  "composite",
  "gemini",
  "grok",
  "openai",
]);

export const supportsImagePricingPlatform = (platform: string): boolean =>
  imagePricingPlatforms.has(platform);

export const imagePricingI18nKey = (_platform: string, key: string): string =>
  `admin.groups.imagePricing.${key}`;
