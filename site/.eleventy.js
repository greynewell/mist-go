const markdownIt = require("markdown-it");

module.exports = function (eleventyConfig) {
  // Markdown with HTML enabled (needed for raw HTML in .md files)
  const md = markdownIt({ html: true, linkify: true, typographer: true });
  eleventyConfig.setLibrary("md", md);

  eleventyConfig.addPassthroughCopy("assets");

  // Flatten a nav structure [{section, pages:[{title,slug}]}] to a flat array
  eleventyConfig.addFilter("flattenNav", (nav) => {
    if (!nav) return [];
    return nav.flatMap((s) => s.pages || []);
  });

  // Given a nav and current fileSlug, return {prev, next}
  eleventyConfig.addFilter("prevNext", (nav, fileSlug) => {
    if (!nav) return { prev: null, next: null };
    const pages = nav.flatMap((s) => s.pages || []);
    const idx = pages.findIndex((p) => p.slug === fileSlug);
    return {
      prev: idx > 0 ? pages[idx - 1] : null,
      next: idx < pages.length - 1 ? pages[idx + 1] : null,
    };
  });

  // Readable date filter
  eleventyConfig.addFilter("readableDate", (date) => {
    return new Date(date).toLocaleDateString("en-US", {
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  });

  return {
    dir: {
      input: ".",
      includes: "_includes",
      data: "_data",
      output: "_site",
    },
    markdownTemplateEngine: "njk",
    htmlTemplateEngine: "njk",
    templateFormats: ["njk", "md"],
  };
};
