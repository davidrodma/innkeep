// O Tailwind CSS 4 roda como um plugin do PostCSS. Não existe mais
// tailwind.config.js: a configuração do tema fica no próprio CSS
// (src/app/globals.css, bloco @theme).
const config = {
  plugins: {
    "@tailwindcss/postcss": {},
  },
};

export default config;
