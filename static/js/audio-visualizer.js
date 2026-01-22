/**
 * Vinylfo Audio Visualizer
 * Canvas-based audio visualization for video feed fallback
 * Simulated mode for OBS (no audio input access)
 */

class AudioVisualizer {
    constructor(canvas, options = {}) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.options = {
            theme: options.theme || 'dark',
            barCount: options.barCount || 64,
            barSpacing: options.barSpacing || 4,
            smoothing: options.smoothing || 0.8,
            minBarHeight: options.minBarHeight || 5,
            reactive: options.reactive !== false
        };

        this.animationId = null;
        this.isRunning = false;
        this.bars = [];
        this.targetBars = [];
        this.time = 0;

        // Theme colors
        this.themes = {
            dark: {
                background: 'rgba(0, 0, 0, 0)',
                barGradient: ['#667eea', '#764ba2', '#f093fb'],
                glow: 'rgba(102, 126, 234, 0.5)'
            },
            light: {
                background: 'rgba(255, 255, 255, 0)',
                barGradient: ['#4facfe', '#00f2fe', '#43e97b'],
                glow: 'rgba(79, 172, 254, 0.5)'
            },
            transparent: {
                background: 'rgba(0, 0, 0, 0)',
                barGradient: ['#ffffff', '#cccccc', '#999999'],
                glow: 'rgba(255, 255, 255, 0.3)'
            }
        };

        this.currentTheme = this.themes[this.options.theme] || this.themes.dark;

        // Initialize bars
        this.initBars();

        // Handle resize
        this.handleResize();
        window.addEventListener('resize', () => this.handleResize());
    }

    initBars() {
        this.bars = new Array(this.options.barCount).fill(0);
        this.targetBars = new Array(this.options.barCount).fill(0);
    }

    handleResize() {
        const rect = this.canvas.parentElement.getBoundingClientRect();
        this.canvas.width = rect.width || window.innerWidth;
        this.canvas.height = rect.height || window.innerHeight;
    }

    start() {
        if (this.isRunning) return;

        this.isRunning = true;
        this.animate();
        console.log('[Visualizer] Started');
    }

    stop() {
        this.isRunning = false;
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
            this.animationId = null;
        }
        console.log('[Visualizer] Stopped');
    }

    animate() {
        if (!this.isRunning) return;

        this.update();
        this.draw();

        this.animationId = requestAnimationFrame(() => this.animate());
    }

    update() {
        this.time += 0.02;

        // Generate simulated audio data using multiple sine waves
        // This creates a realistic-looking audio visualization without actual audio input
        for (let i = 0; i < this.options.barCount; i++) {
            const normalizedIndex = i / this.options.barCount;

            // Combine multiple frequencies for organic movement
            const wave1 = Math.sin(this.time * 2 + normalizedIndex * 8) * 0.3;
            const wave2 = Math.sin(this.time * 3.5 + normalizedIndex * 12) * 0.2;
            const wave3 = Math.sin(this.time * 1.5 + normalizedIndex * 4) * 0.25;
            const wave4 = Math.sin(this.time * 5 + normalizedIndex * 16) * 0.1;

            // Add some randomness for natural feel
            const noise = (Math.random() - 0.5) * 0.1;

            // Bass emphasis (lower frequencies higher)
            const bassBoost = Math.pow(1 - normalizedIndex, 0.5) * 0.3;

            // Combine all components
            let value = 0.4 + wave1 + wave2 + wave3 + wave4 + noise + bassBoost;

            // Add periodic "beats"
            const beatPhase = (this.time * 2) % (Math.PI * 2);
            if (beatPhase < 0.5) {
                value += Math.sin(beatPhase * Math.PI * 2) * 0.3 * (1 - normalizedIndex * 0.5);
            }

            // Clamp to valid range
            this.targetBars[i] = Math.max(0.05, Math.min(1, value));
        }

        // Smooth transition to target values
        for (let i = 0; i < this.options.barCount; i++) {
            this.bars[i] += (this.targetBars[i] - this.bars[i]) * (1 - this.options.smoothing);
        }
    }

    draw() {
        const { width, height } = this.canvas;

        // Clear canvas
        this.ctx.clearRect(0, 0, width, height);

        // Draw background (transparent by default)
        if (this.currentTheme.background !== 'rgba(0, 0, 0, 0)') {
            this.ctx.fillStyle = this.currentTheme.background;
            this.ctx.fillRect(0, 0, width, height);
        }

        // Calculate bar dimensions
        const totalBarWidth = width / this.options.barCount;
        const barWidth = totalBarWidth - this.options.barSpacing;
        const maxBarHeight = height * 0.7;

        // Create gradient
        const gradient = this.ctx.createLinearGradient(0, height, 0, 0);
        this.currentTheme.barGradient.forEach((color, index) => {
            gradient.addColorStop(index / (this.currentTheme.barGradient.length - 1), color);
        });

        // Enable glow effect
        this.ctx.shadowColor = this.currentTheme.glow;
        this.ctx.shadowBlur = 15;

        // Draw bars
        this.ctx.fillStyle = gradient;

        for (let i = 0; i < this.options.barCount; i++) {
            const barHeight = Math.max(
                this.options.minBarHeight,
                this.bars[i] * maxBarHeight
            );

            const x = i * totalBarWidth + this.options.barSpacing / 2;
            const y = height - barHeight;

            // Draw bar with rounded top
            this.drawRoundedBar(x, y, barWidth, barHeight, barWidth / 2);
        }

        // Reset shadow
        this.ctx.shadowBlur = 0;

        // Draw reflection (subtle)
        this.ctx.globalAlpha = 0.15;
        this.ctx.save();
        this.ctx.scale(1, -0.3);
        this.ctx.translate(0, -height * 4);

        for (let i = 0; i < this.options.barCount; i++) {
            const barHeight = Math.max(
                this.options.minBarHeight,
                this.bars[i] * maxBarHeight
            );

            const x = i * totalBarWidth + this.options.barSpacing / 2;
            const y = height - barHeight;

            this.drawRoundedBar(x, y, barWidth, barHeight, barWidth / 2);
        }

        this.ctx.restore();
        this.ctx.globalAlpha = 1;
    }

    drawRoundedBar(x, y, width, height, radius) {
        if (height < radius * 2) {
            radius = height / 2;
        }

        this.ctx.beginPath();
        this.ctx.moveTo(x + radius, y);
        this.ctx.lineTo(x + width - radius, y);
        this.ctx.quadraticCurveTo(x + width, y, x + width, y + radius);
        this.ctx.lineTo(x + width, y + height);
        this.ctx.lineTo(x, y + height);
        this.ctx.lineTo(x, y + radius);
        this.ctx.quadraticCurveTo(x, y, x + radius, y);
        this.ctx.closePath();
        this.ctx.fill();
    }

    setTheme(themeName) {
        if (this.themes[themeName]) {
            this.currentTheme = this.themes[themeName];
            this.options.theme = themeName;
        }
    }

    setBarCount(count) {
        this.options.barCount = count;
        this.initBars();
    }
}

// Make available globally
window.AudioVisualizer = AudioVisualizer;
