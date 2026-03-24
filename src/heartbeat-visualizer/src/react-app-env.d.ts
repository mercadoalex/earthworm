/// <reference types="react-scripts" />

declare module '*.svg' {
  const content: string;
  export default content;
}

declare module '*.mp3' {
  const src: string;
  export default src;
}
