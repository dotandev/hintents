// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import React from "react"; export default ({events,currentTime,onScrub}:any) => <input type="range" min={events[0]?.timestamp||0} max={events[events.length-1]?.timestamp||100} value={currentTime} onChange={e=>onScrub(+e.target.value)} />
