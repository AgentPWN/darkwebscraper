import jwt

token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MTk5OSwiZXhwIjoxNzgxMDA1MjQwLCJ0b2tlbl90eXBlIjoic3R1ZGVudC10b2tlbiJ9.AdC_VeLBrTQREihqVSmV0dyXbm3Ff-6NU1IWvI2lt4k"

payload = jwt.decode(token, options={"verify_signature": False})
print(payload)