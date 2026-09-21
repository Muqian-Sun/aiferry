import { describe, expect, it } from "vitest";

import {
  imagePricingPlatforms,
  imagePricingI18nKey,
  supportsImagePricingPlatform,
} from "../groupsImagePricing";

describe("groups image generation controls platform support", () => {
  it("includes Grok image groups", () => {
    expect(supportsImagePricingPlatform("grok")).toBe(true);
    expect(imagePricingPlatforms.has("grok")).toBe(true);
  });

  it("keeps non-media group platforms out of the image generation controls", () => {
    expect(supportsImagePricingPlatform("anthropic")).toBe(false);
  });

  it("includes Composite groups in image generation controls", () => {
    expect(supportsImagePricingPlatform("composite")).toBe(true);
    expect(imagePricingPlatforms.has("composite")).toBe(true);
  });

  it("uses the image pricing copy namespace", () => {
    expect(imagePricingI18nKey("grok", "title")).toBe(
      "admin.groups.imagePricing.title",
    );
  });
});
