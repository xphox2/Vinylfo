/**
 * Vinylfo Audio Visualizer Pro
 * Multi-mode canvas-based audio visualization for video feed fallback
 * Enhanced with beat detection, particle system, and 6 visualization modes
 */

class AudioVisualizer {
    constructor(canvas, options = {}) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.options = {
            theme: options.theme || 'dark',
            visualizerMode: options.visualizerMode || 'bars',
            barCount: options.barCount || 64,
            barSpacing: options.barSpacing || 4,
            smoothing: options.smoothing || 0.8,
            minBarHeight: options.minBarHeight || 5,
            reactive: options.reactive !== false,
            showParticles: options.showParticles !== false,
            showBeatEffects: options.showBeatEffects !== false,
            sensitivity: options.sensitivity || 1.0,
            particleCount: options.particleCount || 50
        };

        this.animationId = null;
        this.isRunning = false;
        this.bars = [];
        this.targetBars = [];
        this.time = 0;
        this.beatTime = 0;
        this.lastBeat = 0;
        this.beatIntensity = 0;
        this.averageIntensity = 0;

        // Theme configurations with enhanced gradients
        this.themes = {
            dark: {
                background: 'rgba(0, 0, 0, 0)',
                barGradient: ['#667eea', '#764ba2', '#f093fb', '#fa709a'],
                ringGradient: ['#00f5ff', '#00d4ff', '#0099ff', '#667eea'],
                waveformGradient: ['#f093fb', '#fa709a', '#f6d365'],
                particleColors: ['#667eea', '#764ba2', '#f093fb', '#fa709a'],
                glow: 'rgba(102, 126, 234, 0.6)',
                beatGlow: 'rgba(240, 147, 251, 0.8)'
            },
            light: {
                background: 'rgba(255, 255, 255, 0)',
                barGradient: ['#4facfe', '#00f2fe', '#43e97b', '#fa709a'],
                ringGradient: ['#667eea', '#764ba2', '#f093fb'],
                waveformGradient: ['#43e97b', '#4facfe', '#00f2fe'],
                particleColors: ['#4facfe', '#00f2fe', '#43e97b', '#f093fb'],
                glow: 'rgba(79, 172, 254, 0.5)',
                beatGlow: 'rgba(67, 233, 123, 0.6)'
            },
            transparent: {
                background: 'rgba(0, 0, 0, 0)',
                barGradient: ['#ffffff', '#e0e0e0', '#b0b0b0', '#909090'],
                ringGradient: ['#ffffff', '#cccccc', '#999999'],
                waveformGradient: ['#ffffff', '#cccccc', '#999999'],
                particleColors: ['#ffffff', '#e0e0e0', '#b0b0b0'],
                glow: 'rgba(255, 255, 255, 0.4)',
                beatGlow: 'rgba(255, 255, 255, 0.7)'
            },
            neon: {
                background: 'rgba(10, 10, 20, 0)',
                barGradient: ['#ff00ff', '#00ffff', '#ffff00', '#ff00aa'],
                ringGradient: ['#ff00ff', '#00ffff', '#ffff00'],
                waveformGradient: ['#00ffff', '#ff00ff', '#ffff00'],
                particleColors: ['#ff00ff', '#00ffff', '#ffff00', '#ff00aa'],
                glow: 'rgba(255, 0, 255, 0.7)',
                beatGlow: 'rgba(0, 255, 255, 0.9)'
            },
            sunset: {
                background: 'rgba(0, 0, 0, 0)',
                barGradient: ['#ff6b6b', '#feca57', '#ff9ff3', '#54a0ff'],
                ringGradient: ['#ff6b6b', '#feca57', '#ff9ff3'],
                waveformGradient: ['#ff9ff3', '#feca57', '#ff6b6b'],
                particleColors: ['#ff6b6b', '#feca57', '#ff9ff3', '#54a0ff'],
                glow: 'rgba(255, 107, 107, 0.6)',
                beatGlow: 'rgba(254, 202, 87, 0.8)'
            }
        };

        this.currentTheme = this.themes[this.options.theme] || this.themes.dark;

        // Initialize bars
        this.initBars();

        // Initialize particles
        this.initParticles();

        // Handle resize
        this.handleResize();
        window.addEventListener('resize', () => this.handleResize());
    }

    initBars() {
        this.bars = new Array(this.options.barCount).fill(0);
        this.targetBars = new Array(this.options.barCount).fill(0);
    }

    initParticles() {
        this.particles = [];
        for (let i = 0; i < this.options.particleCount; i++) {
            this.particles.push(this.createParticle());
        }
    }

    createParticle() {
        return {
            x: Math.random() * (this.canvas.width || 100),
            y: Math.random() * (this.canvas.height || 100),
            vx: (Math.random() - 0.5) * 2,
            vy: (Math.random() - 0.5) * 2,
            size: Math.random() * 4 + 1,
            alpha: Math.random() * 0.5 + 0.2,
            color: this.currentTheme.particleColors[
                Math.floor(Math.random() * this.currentTheme.particleColors.length)
            ]
        };
    }

    handleResize() {
        const rect = this.canvas.parentElement?.getBoundingClientRect();
        this.canvas.width = rect?.width || window.innerWidth;
        this.canvas.height = rect?.height || window.innerHeight;
        this.initParticles();
    }

    start() {
        if (this.isRunning) return;
        this.isRunning = true;
        this.animate();
        console.log('[Visualizer] Started in mode:', this.options.visualizerMode);
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

        // Generate simulated audio data
        for (let i = 0; i < this.options.barCount; i++) {
            const normalizedIndex = i / this.options.barCount;

            // Combine multiple frequencies for organic movement
            const wave1 = Math.sin(this.time * 2 + normalizedIndex * 8) * 0.3;
            const wave2 = Math.sin(this.time * 3.5 + normalizedIndex * 12) * 0.2;
            const wave3 = Math.sin(this.time * 1.5 + normalizedIndex * 4) * 0.25;
            const wave4 = Math.sin(this.time * 5 + normalizedIndex * 16) * 0.1;
            const noise = (Math.random() - 0.5) * 0.1;
            const bassBoost = Math.pow(1 - normalizedIndex, 0.5) * 0.3;

            let value = 0.4 + wave1 + wave2 + wave3 + wave4 + noise + bassBoost;

            // Add periodic beats
            const beatPhase = (this.time * 2) % (Math.PI * 2);
            if (beatPhase < 0.5) {
                value += Math.sin(beatPhase * Math.PI * 2) * 0.3 * (1 - normalizedIndex * 0.5);
            }

            this.targetBars[i] = Math.max(0.05, Math.min(1, value * this.options.sensitivity));
        }

        // Smooth transition to target values
        for (let i = 0; i < this.options.barCount; i++) {
            this.bars[i] += (this.targetBars[i] - this.bars[i]) * (1 - this.options.smoothing);
        }

        // Calculate beat detection
        this.calculateBeat();

        // Update particles
        if (this.options.showParticles) {
            this.updateParticles();
        }
    }

    calculateBeat() {
        const currentIntensity = this.bars.reduce((a, b) => a + b, 0) / this.bars.length;
        this.averageIntensity = this.averageIntensity * 0.95 + currentIntensity * 0.05;
        
        // Beat detection based on intensity spike
        if (currentIntensity > this.averageIntensity * 1.3 && this.time - this.lastBeat > 0.2) {
            this.lastBeat = this.time;
            this.beatIntensity = Math.min(1, (currentIntensity - this.averageIntensity) * 2);
            this.beatTime = 0;
        }
        
        this.beatTime += 0.05;
        this.beatIntensity *= 0.95; // Decay
    }

    updateParticles() {
        const { width, height } = this.canvas;
        
        this.particles.forEach((p, index) => {
            // Update position
            p.x += p.vx;
            p.y += p.vy;

            // Wrap around edges
            if (p.x < 0) p.x = width;
            if (p.x > width) p.x = 0;
            if (p.y < 0) p.y = height;
            if (p.y > height) p.y = 0;

            // React to beat
            if (this.beatIntensity > 0.3) {
                p.vx += (Math.random() - 0.5) * this.beatIntensity;
                p.vy += (Math.random() - 0.5) * this.beatIntensity;
                p.size = Math.min(8, p.size + 0.5);
            }

            // Slow down and shrink
            p.vx *= 0.99;
            p.vy *= 0.99;
            p.size = Math.max(1, p.size * 0.99);

            // Ensure minimum velocity to prevent particles from stopping completely
            const minVelocity = 0.3;
            const currentVelocity = Math.sqrt(p.vx * p.vx + p.vy * p.vy);
            if (currentVelocity < minVelocity) {
                // Add gentle random movement to keep particles alive
                p.vx += (Math.random() - 0.5) * 0.5;
                p.vy += (Math.random() - 0.5) * 0.5;
            }

            // Color cycling
            if (this.time % 0.1 < 0.02) {
                p.color = this.currentTheme.particleColors[
                    Math.floor(Math.random() * this.currentTheme.particleColors.length)
                ];
            }
        });
    }

    draw() {
        const { width, height } = this.canvas;

        // Clear with fade effect for trails
        this.ctx.fillStyle = 'rgba(0, 0, 0, 0.15)';
        this.ctx.fillRect(0, 0, width, height);

        // Draw based on visualizer mode
        switch (this.options.visualizerMode) {
            case 'bars':
                this.drawBars();
                break;
            case 'circular':
                this.drawCircular();
                break;
            case 'waveform':
                this.drawWaveform();
                break;
            case 'spectrum':
                this.drawSpectrumRing();
                break;
            case 'kaleidoscope':
                this.drawKaleidoscope();
                break;
            case 'particles':
                this.drawParticlesMode();
                break;
            default:
                this.drawBars();
        }

        // Draw particles overlay
        if (this.options.showParticles && this.options.visualizerMode !== 'particles') {
            this.drawParticles();
        }

        // Draw beat effects
        if (this.options.showBeatEffects && this.beatIntensity > 0.1) {
            this.drawBeatEffects();
        }
    }

    drawBars() {
        const { width, height } = this.canvas;
        const totalBarWidth = width / this.options.barCount;
        const barWidth = Math.max(2, totalBarWidth - this.options.barSpacing);
        const maxBarHeight = height * 0.6;

        const gradient = this.ctx.createLinearGradient(0, height, 0, height * 0.2);
        this.currentTheme.barGradient.forEach((color, index) => {
            gradient.addColorStop(index / (this.currentTheme.barGradient.length - 1), color);
        });

        this.ctx.fillStyle = gradient;
        this.ctx.shadowColor = this.currentTheme.glow;
        this.ctx.shadowBlur = 20 + this.beatIntensity * 20;

        for (let i = 0; i < this.options.barCount; i++) {
            const barHeight = Math.max(
                this.options.minBarHeight,
                this.bars[i] * maxBarHeight
            );

            const x = i * totalBarWidth + this.options.barSpacing / 2;
            const y = height - barHeight;

            // Mirror bars from center
            const centerX = width / 2;
            const mirrorX = centerX + (centerX - x - barWidth);

            // Draw bar with glow on peaks
            if (this.bars[i] > 0.8) {
                this.ctx.shadowColor = this.currentTheme.beatGlow;
                this.ctx.shadowBlur = 30;
            } else {
                this.ctx.shadowColor = this.currentTheme.glow;
                this.ctx.shadowBlur = 15;
            }

            this.drawRoundedBar(x, y, barWidth, barHeight, barWidth / 2);
            this.drawRoundedBar(mirrorX, y, barWidth, barHeight, barWidth / 2);
        }

        this.ctx.shadowBlur = 0;

        // Draw reflection
        this.drawReflection(width, height, maxBarHeight, (x, y, w, h, r) => {
            this.drawRoundedBar(x, y, w, h, r);
        });
    }

    drawCircular() {
        const { width, height } = this.canvas;
        const centerX = width / 2;
        const centerY = height / 2;
        const radius = Math.min(width, height) * 0.25;
        const maxRadius = Math.min(width, height) * 0.45;

        this.ctx.save();
        this.ctx.translate(centerX, centerY);
        this.ctx.rotate(this.time * 0.1);

        const gradient = this.ctx.createRadialGradient(0, 0, radius, 0, 0, maxRadius);
        this.currentTheme.ringGradient.forEach((color, index) => {
            gradient.addColorStop(index / (this.currentTheme.ringGradient.length - 1), color);
        });

        this.ctx.fillStyle = gradient;
        this.ctx.shadowColor = this.currentTheme.glow;
        this.ctx.shadowBlur = 20 + this.beatIntensity * 30;

        for (let i = 0; i < this.options.barCount; i++) {
            const angle = (i / this.options.barCount) * Math.PI * 2;
            const barHeight = this.bars[i] * (maxRadius - radius);
            const barWidth = (Math.PI * 2 * radius) / this.options.barCount - 2;

            this.ctx.save();
            this.ctx.rotate(angle);
            
            const x = -barWidth / 2;
            const y = radius;
            
            if (this.bars[i] > 0.8) {
                this.ctx.shadowColor = this.currentTheme.beatGlow;
                this.ctx.shadowBlur = 25;
            }

            this.drawRoundedBar(x, y, barWidth, barHeight, barWidth / 2);
            this.ctx.restore();
        }

        // Inner circle glow
        this.ctx.beginPath();
        this.ctx.arc(0, 0, radius - 10, 0, Math.PI * 2);
        this.ctx.fillStyle = this.currentTheme.glow;
        this.ctx.globalAlpha = 0.3 + this.beatIntensity * 0.4;
        this.ctx.fill();

        this.ctx.restore();
    }

    drawWaveform() {
        const { width, height } = this.canvas;
        const centerY = height / 2;

        this.ctx.save();
        this.ctx.lineWidth = 3;
        this.ctx.lineCap = 'round';
        this.ctx.lineJoin = 'round';

        // Draw multiple waveform lines
        const lines = 3;
        for (let line = 0; line < lines; line++) {
            const gradient = this.ctx.createLinearGradient(0, 0, width, 0);
            const colors = this.currentTheme.waveformGradient;
            colors.forEach((color, index) => {
                gradient.addColorStop(index / (colors.length - 1), color);
            });

            this.ctx.strokeStyle = gradient;
            this.ctx.shadowColor = this.currentTheme.glow;
            this.ctx.shadowBlur = 15 + line * 5 + this.beatIntensity * 20;
            this.ctx.globalAlpha = 1 - line * 0.2;

            this.ctx.beginPath();
            for (let i = 0; i < width; i += 2) {
                const normalizedX = i / width;
                const barIndex = Math.floor(normalizedX * this.options.barCount);
                const amplitude = this.bars[barIndex] * (height * 0.3) * (1 - line * 0.3);
                
                const wave1 = Math.sin(normalizedX * 20 + this.time * 3 + line) * amplitude;
                const wave2 = Math.sin(normalizedX * 10 + this.time * 2) * amplitude * 0.5;
                const y = centerY + wave1 + wave2;

                if (i === 0) {
                    this.ctx.moveTo(i, y);
                } else {
                    this.ctx.lineTo(i, y);
                }
            }
            this.ctx.stroke();
        }

        this.ctx.restore();

        // Draw vertical scope lines
        this.ctx.save();
        this.ctx.strokeStyle = this.currentTheme.glow;
        this.ctx.lineWidth = 1;
        this.ctx.globalAlpha = 0.3;
        
        for (let i = 0; i < width; i += width / 10) {
            this.ctx.beginPath();
            this.ctx.moveTo(i, 0);
            this.ctx.lineTo(i, height);
            this.ctx.stroke();
        }
        this.ctx.restore();
    }

    drawSpectrumRing() {
        const { width, height } = this.canvas;
        const centerX = width / 2;
        const centerY = height / 2;
        const minRadius = Math.min(width, height) * 0.15;
        const maxRadius = Math.min(width, height) * 0.4;

        this.ctx.save();
        this.ctx.translate(centerX, centerY);

        // Draw frequency bands as concentric rings
        const bands = 8;
        for (let band = 0; band < bands; band++) {
            const bandIntensity = this.bars[band * 8] || 0;
            const radius = minRadius + (band / bands) * (maxRadius - minRadius);
            const lineWidth = (maxRadius - minRadius) / bands * 0.8;

            this.ctx.beginPath();
            this.ctx.arc(0, 0, radius, 0, Math.PI * 2);
            
            const gradient = this.ctx.createRadialGradient(0, 0, radius - lineWidth, 0, 0, radius + lineWidth);
            const colors = this.currentTheme.ringGradient;
            const color = colors[band % colors.length];
            const transparentColor = color.startsWith('rgba') 
                ? color.replace(/rgba\(([^,]+),\s*([^,]+),\s*([^,]+),\s*[^)]+\)/, 'rgba($1, $2, $3, 0)')
                : color + '00';
            
            gradient.addColorStop(0, transparentColor);
            gradient.addColorStop(0.5, color);
            gradient.addColorStop(1, transparentColor);
            
            this.ctx.strokeStyle = gradient;
            this.ctx.lineWidth = lineWidth * (0.5 + bandIntensity * 1.5);
            this.ctx.shadowColor = color;
            this.ctx.shadowBlur = 20 + bandIntensity * 30;
            this.ctx.globalAlpha = 0.3 + bandIntensity * 0.7;
            this.ctx.stroke();
        }

        // Rotating beams
        const beamCount = 12;
        this.ctx.globalAlpha = 0.5 + this.beatIntensity * 0.5;
        
        for (let i = 0; i < beamCount; i++) {
            const angle = (i / beamCount) * Math.PI * 2 + this.time * 0.5;
            const barIndex = Math.floor((i / beamCount) * this.options.barCount);
            const intensity = this.bars[barIndex] || 0;
            
            this.ctx.save();
            this.ctx.rotate(angle);
            
            const gradient = this.ctx.createLinearGradient(0, minRadius, 0, maxRadius);
            const glowColor = this.currentTheme.glow;
            const transparentGlow = glowColor.startsWith('rgba')
                ? glowColor.replace(/rgba\(([^,]+),\s*([^,]+),\s*([^,]+),\s*[^)]+\)/, 'rgba($1, $2, $3, 0)')
                : glowColor + '00';
            gradient.addColorStop(0, glowColor);
            gradient.addColorStop(1, transparentGlow);
            
            this.ctx.strokeStyle = gradient;
            this.ctx.lineWidth = 2 + intensity * 4;
            this.ctx.shadowColor = this.currentTheme.beatGlow;
            this.ctx.shadowBlur = 20;
            
            this.ctx.beginPath();
            this.ctx.moveTo(0, minRadius);
            this.ctx.lineTo(0, minRadius + (maxRadius - minRadius) * intensity);
            this.ctx.stroke();
            
            this.ctx.restore();
        }

        this.ctx.restore();
    }

    drawKaleidoscope() {
        const { width, height } = this.canvas;
        const centerX = width / 2;
        const centerY = height / 2;
        const segments = 8;

        this.ctx.save();
        this.ctx.translate(centerX, centerY);

        for (let seg = 0; seg < segments; seg++) {
            this.ctx.save();
            this.ctx.rotate((seg / segments) * Math.PI * 2);

            // Draw mirrored bars
            const barCount = Math.floor(this.options.barCount / 2);
            const maxHeight = Math.min(width, height) * 0.4;

            for (let i = 0; i < barCount; i++) {
                const normalizedIndex = i / barCount;
                const barIndex = Math.floor(normalizedIndex * this.options.barCount);
                const intensity = this.bars[barIndex] || 0;
                
                const barHeight = intensity * maxHeight;
                const barWidth = maxHeight / barCount;
                const x = normalizedIndex * maxHeight;
                const y = -barHeight / 2;

                const gradient = this.ctx.createLinearGradient(x, 0, x + barWidth, 0);
                const colors = this.currentTheme.barGradient;
                gradient.addColorStop(0, colors[0]);
                gradient.addColorStop(1, colors[colors.length - 1]);

                this.ctx.fillStyle = gradient;
                this.ctx.globalAlpha = 0.4 + intensity * 0.6;
                this.ctx.shadowColor = this.currentTheme.glow;
                this.ctx.shadowBlur = 10 + intensity * 20;

                this.drawRoundedBar(x, y, barWidth * 0.8, barHeight, 2);
            }

            this.ctx.restore();
        }

        // Center glow
        this.ctx.beginPath();
        this.ctx.arc(0, 0, 30 + this.beatIntensity * 40, 0, Math.PI * 2);
        this.ctx.fillStyle = this.currentTheme.beatGlow;
        this.ctx.globalAlpha = 0.5 + this.beatIntensity * 0.5;
        this.ctx.shadowColor = this.currentTheme.beatGlow;
        this.ctx.shadowBlur = 50;
        this.ctx.fill();

        this.ctx.restore();
    }

    drawParticlesMode() {
        // Just draw particles without visualizer
        this.ctx.fillStyle = 'rgba(0, 0, 0, 0.1)';
        this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
        this.drawParticles();

        // Add connecting lines between nearby particles
        const { particles } = this;
        const connectionDistance = 100;

        this.ctx.strokeStyle = this.currentTheme.glow;
        this.ctx.lineWidth = 1;

        for (let i = 0; i < particles.length; i++) {
            for (let j = i + 1; j < particles.length; j++) {
                const dx = particles[i].x - particles[j].x;
                const dy = particles[i].y - particles[j].y;
                const distance = Math.sqrt(dx * dx + dy * dy);

                if (distance < connectionDistance) {
                    this.ctx.globalAlpha = (1 - distance / connectionDistance) * 0.3;
                    this.ctx.beginPath();
                    this.ctx.moveTo(particles[i].x, particles[i].y);
                    this.ctx.lineTo(particles[j].x, particles[j].y);
                    this.ctx.stroke();
                }
            }
        }
    }

    drawParticles() {
        const { particles } = this;

        particles.forEach(p => {
            this.ctx.save();
            this.ctx.globalAlpha = p.alpha;
            this.ctx.fillStyle = p.color;
            this.ctx.shadowColor = p.color;
            this.ctx.shadowBlur = 10;
            this.ctx.beginPath();
            this.ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
            this.ctx.fill();
            this.ctx.restore();
        });
    }

    drawBeatEffects() {
        const { width, height } = this.canvas;
        const centerX = width / 2;
        const centerY = height / 2;
        const maxRadius = Math.max(width, height) * 0.8;

        // Shockwave rings
        const rings = 3;
        for (let i = 0; i < rings; i++) {
            const ringProgress = (this.beatTime + i * 0.3) % 1;
            const radius = maxRadius * ringProgress;
            const alpha = this.beatIntensity * (1 - ringProgress);

            this.ctx.save();
            this.ctx.beginPath();
            this.ctx.arc(centerX, centerY, radius, 0, Math.PI * 2);
            this.ctx.strokeStyle = this.currentTheme.beatGlow;
            this.ctx.lineWidth = 5 * this.beatIntensity * (1 - ringProgress);
            this.ctx.globalAlpha = alpha * 0.5;
            this.ctx.stroke();
            this.ctx.restore();
        }

        // Screen flash on strong beats
        if (this.beatIntensity > 0.6) {
            this.ctx.save();
            this.ctx.fillStyle = this.currentTheme.beatGlow;
            this.ctx.globalAlpha = this.beatIntensity * 0.1;
            this.ctx.fillRect(0, 0, width, height);
            this.ctx.restore();
        }

        // Vignette pulse with dynamic alpha
        const vignetteGradient = this.ctx.createRadialGradient(
            centerX, centerY, 0,
            centerX, centerY, maxRadius
        );
        
        // Parse the beatGlow color and create versions with different alpha values
        const beatGlowColor = this.currentTheme.beatGlow;
        // Extract the rgba values and create new strings with dynamic alpha
        const baseColor = beatGlowColor.replace(/rgba?\(([^)]+)\)/, (match, p1) => {
            const parts = p1.split(',').map(s => s.trim());
            return `rgba(${parts[0]}, ${parts[1]}, ${parts[2]}`;
        }).replace(/rgba\(([^)]+)\)/, 'rgba($1)');
        
        // Create color strings with specific alpha values
        const transparentColor = baseColor.replace(/rgba\(([^)]+)\)/, (match, p1) => {
            const parts = p1.split(',').map(s => s.trim());
            return `rgba(${parts[0]}, ${parts[1]}, ${parts[2]}, 0)`;
        });
        
        const pulseColor = baseColor.replace(/rgba\(([^)]+)\)/, (match, p1) => {
            const parts = p1.split(',').map(s => s.trim());
            const alpha = Math.max(0, Math.min(1, this.beatIntensity * 0.5));
            return `rgba(${parts[0]}, ${parts[1]}, ${parts[2]}, ${alpha})`;
        });
        
        vignetteGradient.addColorStop(0, transparentColor);
        vignetteGradient.addColorStop(0.7, transparentColor);
        vignetteGradient.addColorStop(1, pulseColor);

        this.ctx.save();
        this.ctx.fillStyle = vignetteGradient;
        this.ctx.fillRect(0, 0, width, height);
        this.ctx.restore();
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

    drawReflection(width, height, maxBarHeight, drawCallback) {
        this.ctx.save();
        this.ctx.globalAlpha = 0.15;
        this.ctx.scale(1, -0.3);
        this.ctx.translate(0, -height * 4);

        const totalBarWidth = width / this.options.barCount;
        const barWidth = Math.max(2, totalBarWidth - this.options.barSpacing);

        for (let i = 0; i < this.options.barCount; i++) {
            const barHeight = Math.max(
                this.options.minBarHeight,
                this.bars[i] * maxBarHeight
            );

            const x = i * totalBarWidth + this.options.barSpacing / 2;
            const y = height - barHeight;
            const centerX = width / 2;
            const mirrorX = centerX + (centerX - x - barWidth);

            drawCallback(x, y, barWidth, barHeight, barWidth / 2);
            drawCallback(mirrorX, y, barWidth, barHeight, barWidth / 2);
        }

        this.ctx.restore();
        this.ctx.globalAlpha = 1;
    }

    // Public methods for configuration
    setVisualizerMode(mode) {
        if (['bars', 'circular', 'waveform', 'spectrum', 'kaleidoscope', 'particles'].includes(mode)) {
            this.options.visualizerMode = mode;
            console.log('[Visualizer] Mode changed to:', mode);
        }
    }

    setTheme(themeName) {
        if (this.themes[themeName]) {
            this.currentTheme = this.themes[themeName];
            this.options.theme = themeName;
            this.initParticles();
        }
    }

    setBarCount(count) {
        this.options.barCount = Math.max(8, Math.min(256, count));
        this.initBars();
    }

    setParticleCount(count) {
        this.options.particleCount = Math.max(0, Math.min(200, count));
        this.initParticles();
    }

    setSensitivity(sensitivity) {
        this.options.sensitivity = Math.max(0.1, Math.min(3.0, sensitivity));
    }

    toggleParticles(show) {
        this.options.showParticles = show;
    }

    toggleBeatEffects(show) {
        this.options.showBeatEffects = show;
    }
}

// Make available globally
window.AudioVisualizer = AudioVisualizer;
