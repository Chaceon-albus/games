<script setup lang="ts">
import { ref, watch, nextTick, onMounted, computed } from 'vue'
import { useGameState } from './composables/useGameState'

// Initialize game state
const state = useGameState()

// Swapping helper: check if White player is the current user ("Me")
const isWhiteSelf = computed(() => {
  return state.playerWhite.value?.name === state.nickname.value
})

// Game started helper: whether game is currently playing or has ended
const isGameStarted = computed(() => {
  return state.gameStatus.value === 'playing' || state.gameStatus.value === 'ended'
})

// Force light theme
onMounted(() => {
  document.documentElement.classList.remove('dark')
  const observer = new MutationObserver(() => {
    if (document.documentElement.classList.contains('dark')) {
      document.documentElement.classList.remove('dark')
    }
  })
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})

// Nuxt UI Toast service (auto-imported by the Vite plugin)
const toast = useToast()

// Input states
const loginInput = ref(state.nickname.value)
const newRoomInput = ref('')
const chatInput = ref('')
const hoveredCell = ref<{ row: number; col: number }>({ row: 0, col: 0 })
const isHoverActive = ref(false)
const noTransition = ref(false)

const handleCellMouseEnter = (r: number, c: number) => {
  const isEmpty = state.board.value[r][c] === null
  const isPlaying = state.gameStatus.value === 'playing'
  const isPlayer = state.simulationRole.value !== 'spectator'

  if (isEmpty && isPlaying && isPlayer) {
    if (!isHoverActive.value) {
      noTransition.value = true
      hoveredCell.value = { row: r, col: c }
      isHoverActive.value = true
      nextTick(() => {
        noTransition.value = false
      })
    } else {
      hoveredCell.value = { row: r, col: c }
      isHoverActive.value = true
    }
  } else {
    isHoverActive.value = false
  }
}

const handleCellClick = (r: number, c: number) => {
  state.placeStone(r, c)
  isHoverActive.value = false
}

// Sync login input if nickname changes
watch(state.nickname, newVal => {
  loginInput.value = newVal
})

// Auto-scroll chat element
const chatFeedRef = ref<HTMLDivElement | null>(null)
watch(
  () => state.chatMessages.value.length,
  async () => {
    await nextTick()
    if (chatFeedRef.value) {
      chatFeedRef.value.scrollTop = chatFeedRef.value.scrollHeight
    }
  }
)

// Symmetrical Nuxt UI Toast alert triggered by state.winner
watch(
  () => state.winner.value,
  newWinner => {
    if (newWinner) {
      toast.add({
        id: 'win-notification',
        title: '对局结束',
        description: `【${getPlayerNameByColor(newWinner)}】执${newWinner === 'black' ? '黑子' : '白子'}获得胜利！`,
        icon: 'i-heroicons-trophy',
        color: newWinner === 'black' ? 'primary' : 'secondary',
        duration: 5000, // Auto-close after 5 seconds
        'onUpdate:open': (open: boolean) => {
          if (!open) {
            state.resetGameState()
          }
        },
        actions: [
          {
            label: '确认',
            onClick: () => {
              state.resetGameState()
            }
          }
        ]
      })
    } else {
      // Clear toast notification upon board reset
      toast.remove('win-notification')
    }
  }
)

// Trigger handlers
const handleLogin = () => {
  if (loginInput.value.trim()) {
    state.saveNickname(loginInput.value)
  }
}

const handleCreateRoom = () => {
  if (newRoomInput.value.trim()) {
    state.createRoom(newRoomInput.value)
    newRoomInput.value = ''
  }
}

const handleSendChat = () => {
  if (chatInput.value.trim()) {
    state.sendChat(chatInput.value)
    chatInput.value = ''
  }
}

// 15x15 Star Points checker
const isStarPoint = (r: number, c: number) => {
  return (
    (r === 3 && c === 3) ||
    (r === 3 && c === 11) ||
    (r === 7 && c === 7) ||
    (r === 11 && c === 3) ||
    (r === 11 && c === 11)
  )
}

// Helper to check if coordinate is part of the 5-in-a-row winning line
const isWinningCell = (r: number, c: number) => {
  return state.winningLine.value.some(coord => coord.row === r && coord.col === c)
}

// Helper to resolve player name by stone color
const getPlayerNameByColor = (color: 'black' | 'white') => {
  if (color === 'black') {
    return state.playerBlack.value?.name || '黑方玩家'
  }
  return state.playerWhite.value?.name || '白方玩家'
}

// Helper to check if coordinate is the most recently placed stone
const isLastMove = (r: number, c: number) => {
  if (state.history.value.length === 0) return false
  const last = state.history.value[state.history.value.length - 1]
  return last.row === r && last.col === c
}
</script>

<template>
  <UApp :toaster="{ position: 'top-center' }">
    <div
      class="flex flex-col min-h-screen relative z-10 font-sans antialiased transition-colors duration-500"
      :class="[state.currentView.value === 'room' ? 'bg-slate-300' : 'bg-slate-50/40']"
    >
      <!-- Full-screen Vignette Lens Dark Corner Effect (Only in Room battle View) -->
      <div
        v-if="state.currentView.value === 'room'"
        class="pointer-events-none fixed inset-0 z-0 bg-[radial-gradient(circle_at_center,transparent_30%,rgba(0,0,0,0.32)_100%)]"
      ></div>

      <main
        class="flex-grow max-w-7xl w-full mx-auto p-6 flex flex-col justify-start relative z-10"
      >
        <!-- ==========================================================================
             VIEW 1: LOGIN / NICKNAME ENTRANCE
             ========================================================================== -->
        <section
          v-if="state.currentView.value === 'login'"
          class="flex-grow flex items-center justify-center py-10"
        >
          <UCard
            class="w-full max-w-sm shadow-xl border border-slate-100 rounded-2xl p-4 bg-white/90 backdrop-blur-md"
          >
            <h2
              class="text-3xl font-extrabold text-slate-800 text-center tracking-tight mb-6 bg-gradient-to-r from-slate-700 via-slate-900 to-zinc-900 bg-clip-text text-transparent"
            >
              五子棋
            </h2>

            <form class="space-y-5" @submit.prevent="handleLogin">
              <div class="space-y-2">
                <label class="text-sm font-bold text-slate-700 block" for="username">
                  请输入您的玩家昵称
                </label>
                <UInput
                  id="username"
                  v-model="loginInput"
                  type="text"
                  size="lg"
                  icon="i-heroicons-user"
                  placeholder="请输入您的玩家昵称"
                  required
                  maxlength="15"
                  autocomplete="off"
                  class="rounded-xl w-full"
                />
              </div>

              <UButton
                type="submit"
                block
                size="lg"
                color="primary"
                class="font-bold shadow-lg shadow-indigo-200 hover:shadow-indigo-300 transition-all rounded-xl"
                trailing-icon="i-heroicons-arrow-right-20-solid"
              >
                进入游戏
              </UButton>
            </form>
          </UCard>
        </section>

        <!-- ==========================================================================
             VIEW 2: LOBBY / ROOM BROWSER
             ========================================================================== -->
        <section v-else-if="state.currentView.value === 'lobby'" class="space-y-8 py-6">
          <div
            class="flex items-center justify-between flex-wrap gap-4 border-b border-slate-100 pb-6"
          >
            <div class="space-y-1">
              <h2 class="text-2xl font-extrabold text-slate-800 tracking-tight">游戏大厅</h2>
              <p class="text-sm text-slate-500">以下是当前的棋局，您可以随时创建或加入</p>
            </div>

            <div class="flex items-center gap-3">
              <span class="text-sm font-bold text-slate-700 flex items-center gap-1.5">
                <UAvatar
                  :text="state.nickname.value ? state.nickname.value[0].toUpperCase() : '?'"
                  size="xs"
                />
                {{ state.nickname.value }}
              </span>
              <UButton
                color="neutral"
                variant="outline"
                size="sm"
                class="font-bold rounded-xl border-slate-200 hover:bg-slate-50"
                icon="i-heroicons-arrow-right-on-rectangle"
                @click="state.logout"
              >
                离开
              </UButton>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <UInput
              v-model="newRoomInput"
              type="text"
              size="md"
              placeholder="输入一个新房间的名字"
              maxlength="20"
              @keyup.enter="handleCreateRoom"
              class="w-64"
            />
            <UButton
              color="primary"
              size="md"
              class="font-bold rounded-xl"
              icon="i-heroicons-plus"
              @click="handleCreateRoom"
            >
              创建房间
            </UButton>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
            <UCard
              v-for="room in state.rooms"
              :key="room.id"
              class="hover:shadow-xl hover:border-indigo-200 transition-all duration-300 cursor-pointer border border-slate-100 rounded-2xl flex flex-col justify-between"
              @click="state.joinRoom(room)"
            >
              <template #header>
                <div class="flex items-center justify-between">
                  <UBadge
                    :color="
                      room.status === 'waiting'
                        ? 'success'
                        : room.status === 'playing'
                          ? 'primary'
                          : 'warning'
                    "
                    variant="subtle"
                    size="sm"
                    class="font-bold rounded-md"
                  >
                    {{
                      room.status === 'waiting'
                        ? '等待中'
                        : room.status === 'playing'
                          ? '对局中'
                          : '已满员'
                    }}
                  </UBadge>
                  <span class="text-xs font-semibold text-slate-400 flex items-center gap-1">
                    👥 {{ room.playerCount }}/{{ room.maxPlayers }}
                  </span>
                </div>
              </template>

              <h3 class="text-lg font-bold text-slate-800 line-clamp-1 mb-2">{{ room.name }}</h3>

              <template #footer>
                <div class="text-xs text-slate-400">房主：{{ room.creatorName }}</div>
              </template>
            </UCard>
          </div>
        </section>

        <!-- ==========================================================================
             VIEW 3: ROOM / GAME BOARD VIEW
             ========================================================================== -->
        <div
          v-else-if="state.currentView.value === 'room'"
          class="space-y-6 flex-grow flex flex-col justify-center"
        >
          <section class="grid grid-cols-1 xl:grid-cols-12 gap-8 items-start">
            <!-- Left Column: The Flat Go Board -->
            <div class="xl:col-span-7 flex flex-col items-center">
              <div class="board-inner">
                <div class="board-grid-wrapper">
                  <!-- Gliding hover placement indicator (four-corner L-brackets) -->
                  <div
                    class="hover-corner-indicator"
                    :class="{
                      'is-active': isHoverActive,
                      'no-transition': noTransition
                    }"
                    :style="{
                      left: `${(hoveredCell.col * 100) / state.boardSize}%`,
                      top: `${(hoveredCell.row * 100) / state.boardSize}%`,
                      width: `${100 / state.boardSize}%`,
                      height: `${100 / state.boardSize}%`
                    }"
                  >
                    <svg
                      viewBox="0 0 24 24"
                      fill="none"
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-full h-full"
                    >
                      <!-- Top Left Corner -->
                      <path
                        d="M6 2H2V6"
                        stroke="currentColor"
                        stroke-width="1.2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                      <!-- Top Right Corner -->
                      <path
                        d="M18 2H22V6"
                        stroke="currentColor"
                        stroke-width="1.2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                      <!-- Bottom Left Corner -->
                      <path
                        d="M6 22H2V18"
                        stroke="currentColor"
                        stroke-width="1.2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                      <!-- Bottom Right Corner -->
                      <path
                        d="M18 22H22V18"
                        stroke="currentColor"
                        stroke-width="1.2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                  </div>

                  <div class="board-grid" @mouseleave="isHoverActive = false">
                    <div v-for="r in state.boardSize" :key="r" class="board-row">
                      <div
                        v-for="c in state.boardSize"
                        :key="c"
                        class="board-cell"
                        @click="handleCellClick(r - 1, c - 1)"
                        @mouseenter="handleCellMouseEnter(r - 1, c - 1)"
                      >
                        <!-- Star point coordinates overlay -->
                        <div v-if="isStarPoint(r - 1, c - 1)" class="star-point-dot"></div>

                        <!-- Placed spherical 3D stone -->
                        <div
                          v-if="state.board.value[r - 1][c - 1] !== null"
                          :class="[
                            'stone',
                            state.board.value[r - 1][c - 1],
                            'stone-placed-animation'
                          ]"
                        >
                          <!-- Last move indicator (subtle glowing red center dot) -->
                          <div v-if="isLastMove(r - 1, c - 1)" class="last-move-indicator"></div>
                        </div>

                        <!-- 5-in-a-row Win highlight overlay (perfectly centered to board-cell) -->
                        <div v-if="isWinningCell(r - 1, c - 1)" class="winning-highlight"></div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Right Column: Symmetrical Match Hub Card powered by Nuxt UI UCard -->
            <div class="xl:col-span-5 w-full">
              <UCard
                :ui="{
                  root: 'match-hub-card flex flex-col shadow-xl border border-slate-100 rounded-3xl bg-white/90 backdrop-blur-md overflow-hidden',
                  body: 'flex-grow flex flex-col min-h-0 p-4 sm:p-6',
                  footer: 'p-4 sm:p-6 border-t border-slate-100'
                }"
              >
                <div class="space-y-6 flex-grow flex flex-col min-h-0">
                  <div class="flex items-center justify-between gap-2">
                    <!-- Black Player -->
                    <div
                      :class="[
                        'flex-1 min-w-0 grid grid-cols-[auto_1fr] gap-x-1.5 sm:gap-x-2 gap-y-1 items-center p-2 sm:p-3 rounded-2xl border transition-all duration-300',
                        state.turn.value === 'black' && state.gameStatus.value === 'playing'
                          ? 'border-indigo-500 bg-indigo-50/30 ring-2 ring-indigo-500/20 shadow-sm'
                          : 'border-slate-200 bg-slate-50/50 shadow-sm',
                        isWhiteSelf ? 'order-3' : 'order-1'
                      ]"
                    >
                      <UAvatar
                        :text="
                          state.playerBlack.value?.name
                            ? state.playerBlack.value.name[0].toUpperCase()
                            : '?'
                        "
                        size="sm"
                        class="flex-shrink-0"
                      />
                      <span class="text-sm font-bold text-slate-800 truncate">
                        {{ state.playerBlack.value?.name || '等待加入...' }}
                      </span>
                      <span
                        class="inline-block w-3 h-3 rounded-full bg-gradient-to-br from-slate-700 to-slate-900 border border-slate-950 shadow-sm justify-self-center"
                      ></span>
                      <div class="flex items-center gap-1.5">
                        <UBadge
                          v-if="isGameStarted"
                          color="neutral"
                          variant="solid"
                          size="sm"
                          class="font-bold rounded-md shadow-sm"
                        >
                          执黑
                        </UBadge>
                        <UBadge
                          v-else
                          :color="state.playerBlack.value?.isReady ? 'success' : 'neutral'"
                          :variant="state.playerBlack.value?.isReady ? 'solid' : 'subtle'"
                          size="sm"
                          :class="[
                            'font-bold rounded-md shadow-sm',
                            !state.playerBlack.value?.isReady
                              ? 'text-slate-500 bg-slate-100 border border-slate-200'
                              : ''
                          ]"
                        >
                          {{ state.playerBlack.value?.isReady ? '已准备' : '等待中' }}
                        </UBadge>
                        <span
                          v-if="state.playerBlack.value?.name === state.nickname.value"
                          class="text-xs font-bold text-indigo-500"
                        >
                          (我)
                        </span>
                      </div>
                    </div>

                    <div class="text-sm font-black text-slate-400 opacity-60 order-2 flex-shrink-0">
                      VS
                    </div>

                    <!-- White Player -->
                    <div
                      :class="[
                        'flex-1 min-w-0 grid grid-cols-[auto_1fr] gap-x-1.5 sm:gap-x-2 gap-y-1 items-center p-2 sm:p-3 rounded-2xl border transition-all duration-300',
                        state.turn.value === 'white' && state.gameStatus.value === 'playing'
                          ? 'border-purple-500 bg-purple-50/30 ring-2 ring-purple-500/20 shadow-sm'
                          : 'border-slate-200 bg-slate-50/50 shadow-sm',
                        isWhiteSelf ? 'order-1' : 'order-3'
                      ]"
                    >
                      <UAvatar
                        :text="
                          state.playerWhite.value?.name
                            ? state.playerWhite.value.name[0].toUpperCase()
                            : '?'
                        "
                        size="sm"
                        class="flex-shrink-0"
                      />
                      <span class="text-sm font-bold text-slate-800 truncate">
                        {{ state.playerWhite.value?.name || '等待加入...' }}
                      </span>
                      <span
                        class="inline-block w-3 h-3 rounded-full bg-gradient-to-br from-slate-50 to-slate-200 border border-slate-300 shadow-sm justify-self-center"
                      ></span>
                      <div class="flex items-center gap-1.5">
                        <UBadge
                          v-if="isGameStarted"
                          color="neutral"
                          variant="outline"
                          size="sm"
                          class="font-bold rounded-md shadow-sm bg-white border-slate-300 text-slate-700"
                        >
                          执白
                        </UBadge>
                        <UBadge
                          v-else
                          :color="state.playerWhite.value?.isReady ? 'success' : 'neutral'"
                          :variant="state.playerWhite.value?.isReady ? 'solid' : 'subtle'"
                          size="sm"
                          :class="[
                            'font-bold rounded-md shadow-sm',
                            !state.playerWhite.value?.isReady
                              ? 'text-slate-500 bg-slate-100 border border-slate-200'
                              : ''
                          ]"
                        >
                          {{ state.playerWhite.value?.isReady ? '已准备' : '等待中' }}
                        </UBadge>
                        <span
                          v-if="state.playerWhite.value?.name === state.nickname.value"
                          class="text-xs font-bold text-purple-500"
                        >
                          (我)
                        </span>
                      </div>
                    </div>
                  </div>

                  <!-- Spectators bar -->
                  <div
                    class="hidden md:flex items-center gap-2 text-xs text-slate-500 bg-slate-50 border border-slate-100 p-2.5 rounded-xl"
                  >
                    <span class="font-bold">👥 观战中 ({{ state.spectators.value.length }})：</span>
                    <span class="font-semibold text-slate-700 truncate max-w-[280px]">
                      {{ state.spectators.value.map(s => s.name).join(', ') || '暂无观战人员' }}
                    </span>
                  </div>

                  <!-- Live Chat Box Container -->
                  <div
                    class="hidden md:flex flex-col border border-slate-100 rounded-2xl overflow-hidden bg-slate-50/50 flex-grow min-h-0"
                  >
                    <div
                      ref="chatFeedRef"
                      class="flex-grow p-4 overflow-y-auto space-y-3 flex flex-col"
                    >
                      <div
                        v-for="msg in state.chatMessages.value"
                        :key="msg.id"
                        :class="[
                          'flex flex-col max-w-[80%] rounded-2xl px-3 py-2 text-sm leading-relaxed shadow-sm',
                          msg.isSystem
                            ? 'self-center max-w-[90%] text-xs italic bg-slate-100 text-slate-500 rounded-xl py-1 border-0 shadow-none'
                            : msg.senderName === state.nickname.value
                              ? 'self-end bg-indigo-600 text-white rounded-tr-none'
                              : 'self-start bg-white text-slate-800 border border-slate-100 rounded-tl-none'
                        ]"
                      >
                        <span v-if="!msg.isSystem" class="text-[10px] font-bold opacity-60 mb-0.5">
                          {{ msg.senderName }} ({{ msg.timestamp }})
                        </span>
                        <span class="break-all">{{ msg.text }}</span>
                      </div>
                    </div>

                    <!-- Chat Input bar -->
                    <form
                      class="flex border-t border-slate-100 bg-white"
                      @submit.prevent="handleSendChat"
                    >
                      <input
                        v-model="chatInput"
                        type="text"
                        class="flex-grow border-0 px-4 py-3 outline-none text-sm text-slate-800 disabled:bg-slate-50 disabled:text-slate-400 disabled:italic rounded-bl-2xl"
                        :placeholder="
                          state.simulationRole.value === 'spectator' ? '观棋不语真君子' : '说点什么'
                        "
                        :disabled="state.simulationRole.value === 'spectator'"
                        maxlength="100"
                      />
                      <UButton
                        type="submit"
                        variant="ghost"
                        color="neutral"
                        size="sm"
                        :disabled="state.simulationRole.value === 'spectator' || !chatInput.trim()"
                        class="font-bold px-4 rounded-none rounded-br-2xl"
                      >
                        发送
                      </UButton>
                    </form>
                  </div>
                </div>

                <!-- Dashboard Symmetrical Actions Panel powered by Nuxt UI UButton -->
                <template #footer>
                  <div class="grid grid-cols-2 gap-3 sm:gap-4">
                    <!-- 悔棋 -->
                    <UButton
                      block
                      color="neutral"
                      variant="solid"
                      size="md"
                      class="font-bold rounded-xl shadow-sm border border-slate-200"
                      :disabled="
                        state.simulationRole.value === 'spectator' ||
                        state.history.value.length === 0
                      "
                      @click="state.retractMove"
                    >
                      悔棋
                    </UButton>

                    <!-- 认输 -->
                    <UButton
                      block
                      color="danger"
                      variant="solid"
                      size="md"
                      class="font-bold rounded-xl shadow-sm shadow-red-50"
                      :disabled="
                        state.simulationRole.value === 'spectator' ||
                        state.gameStatus.value !== 'playing'
                      "
                      @click="state.resignGame"
                    >
                      认输
                    </UButton>

                    <!-- 准备 -->
                    <UButton
                      block
                      color="primary"
                      :variant="
                        state.gameStatus.value === 'playing'
                          ? 'soft'
                          : state.simulationRole.value !== 'spectator' &&
                              ((state.simulationRole.value === 'black' &&
                                state.playerBlack.value?.isReady) ||
                                (state.simulationRole.value === 'white' &&
                                  state.playerWhite.value?.isReady))
                            ? 'soft'
                            : 'solid'
                      "
                      size="md"
                      class="font-bold rounded-xl shadow-md shadow-indigo-50"
                      :disabled="
                        state.simulationRole.value === 'spectator' ||
                        state.gameStatus.value === 'playing'
                      "
                      @click="state.toggleReady"
                    >
                      {{
                        state.gameStatus.value === 'playing'
                          ? '对局中'
                          : state.simulationRole.value !== 'spectator' &&
                              ((state.simulationRole.value === 'black' &&
                                state.playerBlack.value?.isReady) ||
                                (state.simulationRole.value === 'white' &&
                                  state.playerWhite.value?.isReady))
                            ? '取消准备'
                            : '准备'
                      }}
                    </UButton>

                    <!-- 离开房间 -->
                    <UButton
                      block
                      color="neutral"
                      variant="outline"
                      size="md"
                      class="font-bold rounded-xl border border-slate-200 hover:bg-slate-50"
                      :disabled="
                        state.simulationRole.value !== 'spectator' &&
                        state.gameStatus.value === 'playing'
                      "
                      @click="state.leaveRoom"
                    >
                      离开房间
                    </UButton>
                  </div>
                </template>
              </UCard>
            </div>
          </section>

          <!-- Role switcher centered at the bottom, spanning across the page -->
          <div class="flex justify-center w-full">
            <UButtonGroup
              class="shadow-md border border-slate-100 rounded-full p-1 bg-white/80 backdrop-blur-md"
            >
              <UButton
                :variant="state.simulationRole.value === 'black' ? 'solid' : 'ghost'"
                color="primary"
                size="sm"
                class="rounded-full font-bold px-5"
                @click="state.simulationRole.value = 'black'"
              >
                <span
                  class="inline-block w-2.5 h-2.5 rounded-full bg-black ring-1 ring-white/50 mr-2"
                ></span>
                执黑玩家
              </UButton>
              <UButton
                :variant="state.simulationRole.value === 'white' ? 'solid' : 'ghost'"
                color="neutral"
                size="sm"
                class="rounded-full font-bold px-5"
                @click="state.simulationRole.value = 'white'"
              >
                <span
                  class="inline-block w-2.5 h-2.5 rounded-full bg-white ring-1 ring-slate-300 mr-2"
                ></span>
                执白玩家
              </UButton>
              <UButton
                :variant="state.simulationRole.value === 'spectator' ? 'solid' : 'ghost'"
                color="success"
                size="sm"
                class="rounded-full font-bold px-5"
                @click="state.simulationRole.value = 'spectator'"
              >
                <span class="inline-block w-2.5 h-2.5 rounded-full bg-emerald-500 mr-2"></span>
                局外人
              </UButton>
            </UButtonGroup>
          </div>
        </div>
      </main>
    </div>
  </UApp>
</template>
