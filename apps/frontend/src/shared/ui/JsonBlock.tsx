import React from "react";
import { styles } from "./styles";

export function JsonBlock({ value }: { value: unknown }) {
  return <pre style={styles.pre}>{JSON.stringify(value, null, 2)}</pre>;
}