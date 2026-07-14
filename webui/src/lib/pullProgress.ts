/**
 * Aggregates per-layer byte counters from docker pull progressDetail into
 * a real percentage, mapped to the 5–70 band of the overall progress bar
 * (the remaining 30% covers stop/recreate/start).
 */
export interface LayerProgress {
  update(layerId: string | undefined, current?: number, total?: number): number | null
}

export function createLayerProgress(): LayerProgress {
  const layers = new Map<string, { current: number; total: number }>()
  return {
    update(layerId, current, total) {
      if (!layerId || !total) return null
      const prev = layers.get(layerId)
      layers.set(layerId, {
        current: Math.max(current ?? 0, prev?.current ?? 0),
        total,
      })
      let cur = 0
      let tot = 0
      for (const l of layers.values()) {
        cur += Math.min(l.current, l.total)
        tot += l.total
      }
      if (tot === 0) return null
      return 5 + Math.round((cur / tot) * 65)
    },
  }
}
