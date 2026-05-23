<script setup lang="ts">
import gomokuCover from './assets/gomoku_cover.png'

// Automatically handle the copyright year dynamically
const currentYear = new Date().getFullYear()
</script>

<template>
  <!-- Main Grid Selection Layout -->
  <main class="main-content">
    <div class="games-grid">
      <!-- Active Game: Gomoku -->
      <a href="/gomoku" class="game-card">
        <div class="card-image-wrapper">
          <img :src="gomokuCover" alt="五子棋" class="game-image" />
          <div class="card-image-overlay"></div>
        </div>

        <div class="card-content">
          <div class="game-header">
            <h3 class="game-title">五子棋</h3>
          </div>

          <p class="game-description">胜固欣然，败亦可喜。来盘方寸博弈，且看谁先落子成线。</p>
        </div>
      </a>

      <!-- Coming Soon Placeholder Card with Quote -->
      <div class="coming-soon-card">
        <div class="placeholder-icon-container">
          <svg
            class="sparkle-svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M6 3h12l4 6-10 12L2 9z"></path>
            <path d="M11 3 8 9l4 12 4-12-3-6"></path>
            <path d="M2 9h20"></path>
          </svg>
        </div>

        <h3 class="placeholder-title">更多内容敬请期待</h3>

        <div class="quote-divider"></div>

        <p class="coming-soon-quote">“面包会有的，牛奶会有的，一切都会有的。”</p>
      </div>
    </div>
  </main>

  <!-- Fully Blended Dynamic Footer -->
  <footer class="footer">
    <div class="footer-copy">© {{ currentYear }} Crystal Games. All Rights Reserved.</div>
  </footer>
</template>
