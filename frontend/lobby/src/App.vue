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

          <p class="game-description">胜固欣然，败亦可喜。方寸博弈，看谁先落子成线。</p>
        </div>
      </a>

      <!-- Coming Soon Placeholder Card with Quote -->
      <div class="coming-soon-card">
        <div class="placeholder-icon-container">
          <img src="/favicon.svg" alt="更多内容敬请期待" class="sparkle-svg" />
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
