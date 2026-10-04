/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export function classNames(...classes: (string | boolean | undefined | null)[]): string {
  return classes.filter(Boolean).join(" ");
}

// basePath gives the base path that the server wrote into <base href>, with no
// trailing slash: "" at the root, or "/dashbrr". Use it only where a URL must
// be absolute. Relative URLs resolve against <base> without help.
export function basePath(): string {
  return new URL(document.baseURI).pathname.replace(/\/$/, "");
}
