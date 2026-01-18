import sys

with open('E:\\Golang\\OpenCode\\Vinylfo\\static\\js\\playlist.js', 'r') as f:
    lines = f.readlines()

balance = 0
for i, line in enumerate(lines, 1):
    balance += line.count('{')
    balance -= line.count('}')
    if balance < 0:
        print(f"Negative balance at line {i}: {balance}")
        break
    if balance == 0 and i > 700:
        # Check if this might be the end
        pass

if balance > 0:
    print(f"Missing {balance} closing braces at end of file")
else:
    print(f"Balance is {balance}")
