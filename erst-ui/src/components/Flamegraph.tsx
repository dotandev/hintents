// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

import React from 'react';

interface FlamegraphProps {
  maxStackDepth: number;
}

export const Flamegraph: React.FC<FlamegraphProps> = ({ maxStackDepth }) => {
  // Dynamically calculate canvas height based on max stack depth to prevent truncation
  const canvasHeight = maxStackDepth * 20 + 50;

  return (
    <svg height={canvasHeight}>
      {/* SVG content for recursive contracts */}
    </svg>
  );
};
