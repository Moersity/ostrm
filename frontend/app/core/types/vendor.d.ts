declare module 'vue-select' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, unknown>>
  export default component
}
declare module 'vue-virtual-scroller' {
  import type { DefineComponent } from 'vue'
  export const RecycleScroller: DefineComponent<Record<string, unknown>>
  export const DynamicScroller: DefineComponent<Record<string, unknown>>
  export const DynamicScrollerItem: DefineComponent<Record<string, unknown>>
}
