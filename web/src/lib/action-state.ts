// Estado das Server Actions — em um módulo sem "server-only" porque os
// componentes de CLIENTE também precisam do tipo e do valor inicial.

export type ActionState = {
  ok?: boolean;
  message?: string;
  errors?: Record<string, string>;
};

export const initialState: ActionState = {};
