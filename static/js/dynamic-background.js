/**
 * Vinylfo Dynamic Background Effects
 * Animated mesh gradients, flowing patterns, and noise overlays
 */

class DynamicBackground {
    constructor(canvas, options = {}) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.options = {
            mode: options.mode || 'mesh', // mesh, flow, noise, aurora
            theme: options.theme || 'dark',
            speed: options.speed || 1.0,
            intensity: options.intensity || 0.5,
            blurAmount: options.blurAmount || 30
        };

        this.animationId = null;
        this.isRunning = false;
        this.time = 0;
        this.noiseSeed = Math.random() * 1000;

        // Mesh gradient control points
        this.meshPoints = [];

        // Flow particles
        this.flowParticles = [];

        // Theme colors
        this.themes = {
            dark: {
                colors: ['#1a1a2e', '#16213e', '#0f0f23', '#1a1a3e', '#0d1b2a'],
                accent: '#667eea'
            },
            light: {
                colors: ['#f5f7fa', '#e4e8eb', '#f0f3f5', '#f8fafc', '#e8ecef'],
                accent: '#4facfe'
            },
            transparent: {
                colors: ['#000000', '#111111', '#0a0a0a', '#151515', '#050505'],
                accent: '#ffffff'
            },
            neon: {
                colors: ['#0a0a14', '#0f0a1e', '#0a1428', '#140a1e', '#0a1e14'],
                accent: '#ff00ff'
            },
            sunset: {
                colors: ['#2d1b2e', '#3d1f2d', '#1f2d3d', '#2e1d2b', '#1d2b3d'],
                accent: '#ff6b6b'
            }
        };

        this.currentTheme = this.themes[this.options.theme] || this.themes.dark;

        // Initialize mesh points and flow particles after theme is set
        this.initMeshPoints();
        this.initFlowParticles();

        this.handleResize();
        window.addEventListener('resize', () => this.handleResize());
    }

    initMeshPoints() {
        // Create control points for mesh gradient
        const pointCount = 4;
        this.meshPoints = [];
        
        for (let i = 0; i < pointCount; i++) {
            this.meshPoints.push({
                x: Math.random(),
                y: Math.random(),
                vx: (Math.random() - 0.5) * 0.002 * this.options.speed,
                vy: (Math.random() - 0.5) * 0.002 * this.options.speed,
                color: this.currentTheme.colors[i % this.currentTheme.colors.length],
                radius: 0.3 + Math.random() * 0.4
            });
        }
    }

    initFlowParticles() {
        this.flowParticles = [];
        const particleCount = 20;
        
        for (let i = 0; i < particleCount; i++) {
            this.flowParticles.push({
                x: Math.random() * (this.canvas.width || 100),
                y: Math.random() * (this.canvas.height || 100),
                angle: Math.random() * Math.PI * 2,
                speed: 0.5 + Math.random() * 1.5,
                size: 50 + Math.random() * 150,
                color: this.currentTheme.colors[Math.floor(Math.random() * this.currentTheme.colors.length)],
                opacity: 0.1 + Math.random() * 0.3
            });
        }
    }

    handleResize() {
        const rect = this.canvas.parentElement?.getBoundingClientRect();
        this.canvas.width = rect?.width || window.innerWidth;
        this.canvas.height = rect?.height || window.innerHeight;
        this.initFlowParticles();
    }

    start() {
        if (this.isRunning) return;
        this.isRunning = true;
        this.animate();
        console.log('[DynamicBackground] Started in mode:', this.options.mode);
    }

    stop() {
        this.isRunning = false;
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
            this.animationId = null;
        }
        console.log('[DynamicBackground] Stopped');
    }

    animate() {
        if (!this.isRunning) return;
        this.update();
        this.draw();
        this.animationId = requestAnimationFrame(() => this.animate());
    }

    update() {
        this.time += 0.01 * this.options.speed;

        switch (this.options.mode) {
            case 'mesh':
                this.updateMeshPoints();
                break;
            case 'flow':
                this.updateFlowParticles();
                break;
            case 'aurora':
                this.updateAurora();
                break;
        }
    }

    updateMeshPoints() {
        this.meshPoints.forEach(point => {
            point.x += point.vx;
            point.y += point.vy;

            // Bounce off edges
            if (point.x < -0.2 || point.x > 1.2) point.vx *= -1;
            if (point.y < -0.2 || point.y > 1.2) point.vy *= -1;
        });
    }

    updateFlowParticles() {
        this.flowParticles.forEach(p => {
            // Move in flowing pattern
            p.angle += 0.01;
            p.x += Math.cos(p.angle) * p.speed * this.options.speed;
            p.y += Math.sin(p.angle * 0.7) * p.speed * 0.5 * this.options.speed;

            // Wrap around screen
            if (p.x < -p.size) p.x = this.canvas.width + p.size;
            if (p.x > this.canvas.width + p.size) p.x = -p.size;
            if (p.y < -p.size) p.y = this.canvas.height + p.size;
            if (p.y > this.canvas.height + p.size) p.y = -p.size;
        });
    }

    updateAurora() {
        // Aurora uses time-based sine waves, no point updates needed
    }

    draw() {
        const { width, height } = this.canvas;

        // Clear with slight fade for trails
        this.ctx.fillStyle = this.currentTheme.colors[0];
        this.ctx.fillRect(0, 0, width, height);

        switch (this.options.mode) {
            case 'mesh':
                this.drawMeshGradient();
                break;
            case 'flow':
                this.drawFlowGradient();
                break;
            case 'noise':
                this.drawNoise();
                break;
            case 'aurora':
                this.drawAurora();
                break;
        }

        // Apply blur for softness
        if (this.options.blurAmount > 0) {
            this.ctx.filter = `blur(${this.options.blurAmount}px)`;
            const tempCanvas = document.createElement('canvas');
            tempCanvas.width = width;
            tempCanvas.height = height;
            const tempCtx = tempCanvas.getContext('2d');
            tempCtx.drawImage(this.canvas, 0, 0);
            this.ctx.clearRect(0, 0, width, height);
            this.ctx.drawImage(tempCanvas, 0, 0);
            this.ctx.filter = 'none';
        }
    }

    drawMeshGradient() {
        const { width, height } = this.canvas;
        const imageData = this.ctx.createImageData(width, height);
        const data = imageData.data;

        // Simplified mesh gradient using radial gradients from control points
        this.meshPoints.forEach((point, index) => {
            const x = point.x * width;
            const y = point.y * height;
            const radius = point.radius * Math.max(width, height);

            const gradient = this.ctx.createRadialGradient(x, y, 0, x, y, radius);
            const baseColor = this.hexToRgb(point.color);
            
            gradient.addColorStop(0, `rgba(${baseColor.r}, ${baseColor.g}, ${baseColor.b}, ${this.options.intensity * 0.8})`);
            gradient.addColorStop(1, `rgba(${baseColor.r}, ${baseColor.g}, ${baseColor.b}, 0)`);

            this.ctx.fillStyle = gradient;
            this.ctx.fillRect(0, 0, width, height);
        });
    }

    drawFlowGradient() {
        const { width, height } = this.canvas;

        // Draw flowing orbs
        this.flowParticles.forEach(p => {
            const gradient = this.ctx.createRadialGradient(
                p.x, p.y, 0,
                p.x, p.y, p.size
            );
            
            const baseColor = this.hexToRgb(p.color);
            gradient.addColorStop(0, `rgba(${baseColor.r}, ${baseColor.g}, ${baseColor.b}, ${p.opacity * this.options.intensity})`);
            gradient.addColorStop(0.5, `rgba(${baseColor.r}, ${baseColor.g}, ${baseColor.b}, ${p.opacity * 0.5 * this.options.intensity})`);
            gradient.addColorStop(1, `rgba(${baseColor.r}, ${baseColor.g}, ${baseColor.b}, 0)`);

            this.ctx.fillStyle = gradient;
            this.ctx.fillRect(0, 0, width, height);
        });
    }

    drawNoise() {
        const { width, height } = this.canvas;
        const imageData = this.ctx.createImageData(width, height);
        const data = imageData.data;

        // Generate organic noise pattern
        for (let y = 0; y < height; y += 2) {
            for (let x = 0; x < width; x += 2) {
                const noise = this.simplexNoise(
                    (x + this.time * 50) * 0.005,
                    (y + this.time * 30) * 0.005
                );

                const baseColor = this.hexToRgb(this.currentTheme.colors[0]);
                const accentColor = this.hexToRgb(this.currentTheme.accent);

                const r = Math.floor(baseColor.r + (accentColor.r - baseColor.r) * noise * this.options.intensity);
                const g = Math.floor(baseColor.g + (accentColor.g - baseColor.g) * noise * this.options.intensity);
                const b = Math.floor(baseColor.b + (accentColor.b - baseColor.b) * noise * this.options.intensity);

                const index = (y * width + x) * 4;
                data[index] = r;
                data[index + 1] = g;
                data[index + 2] = b;
                data[index + 3] = 255;

                // Fill adjacent pixels
                if (x + 1 < width) {
                    data[index + 4] = r;
                    data[index + 5] = g;
                    data[index + 6] = b;
                    data[index + 7] = 255;
                }
            }
        }

        this.ctx.putImageData(imageData, 0, 0);
    }

    drawAurora() {
        const { width, height } = this.canvas;

        // Create aurora-like wave patterns
        const waveCount = 3;
        
        for (let wave = 0; wave < waveCount; wave++) {
            const gradient = this.ctx.createLinearGradient(0, 0, width, 0);
            const colors = this.currentTheme.colors.slice(1, 4);
            
            colors.forEach((color, index) => {
                gradient.addColorStop(index / colors.length, color);
            });

            this.ctx.fillStyle = gradient;
            this.ctx.globalAlpha = this.options.intensity * 0.3;

            this.ctx.beginPath();
            this.ctx.moveTo(0, height);

            for (let x = 0; x <= width; x += 5) {
                const normalizedX = x / width;
                const waveOffset = wave * Math.PI * 0.5;
                const y = height * 0.5 + 
                    Math.sin(normalizedX * Math.PI * 4 + this.time + waveOffset) * height * 0.2 +
                    Math.sin(normalizedX * Math.PI * 2 + this.time * 0.5 + waveOffset) * height * 0.1;
                
                if (x === 0) {
                    this.ctx.moveTo(x, y);
                } else {
                    this.ctx.lineTo(x, y);
                }
            }

            this.ctx.lineTo(width, height);
            this.ctx.lineTo(0, height);
            this.ctx.closePath();
            this.ctx.fill();
        }

        this.ctx.globalAlpha = 1;
    }

    // Simple pseudo-random noise function
    simplexNoise(x, y) {
        return (Math.sin(x * 12.9898 + y * 78.233 + this.noiseSeed) * 43758.5453 % 1) * 0.5 + 0.5;
    }

    hexToRgb(hex) {
        const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
        return result ? {
            r: parseInt(result[1], 16),
            g: parseInt(result[2], 16),
            b: parseInt(result[3], 16)
        } : { r: 0, g: 0, b: 0 };
    }

    // Public methods
    setMode(mode) {
        if (['mesh', 'flow', 'noise', 'aurora'].includes(mode)) {
            this.options.mode = mode;
            this.initMeshPoints();
            this.initFlowParticles();
        }
    }

    setTheme(themeName) {
        if (this.themes[themeName]) {
            this.currentTheme = this.themes[themeName];
            this.options.theme = themeName;
            this.initMeshPoints();
            this.initFlowParticles();
        }
    }

    setSpeed(speed) {
        this.options.speed = Math.max(0.1, Math.min(5.0, speed));
    }

    setIntensity(intensity) {
        this.options.intensity = Math.max(0.1, Math.min(1.0, intensity));
    }
}

// Make available globally
window.DynamicBackground = DynamicBackground;
